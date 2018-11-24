// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The buildlet is an HTTP server that untars content to disk and runs
// commands it has untarred, streaming their output back over HTTP.
// It is part of Go's continuous build system.
//
// This program intentionally allows remote code execution, and
// provides no security of its own. It is assumed that any user uses
// it with an appropriately-configured firewall between their VM
// instances.
package main // import "golang.org/x/build/cmd/buildlet"

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/compute/metadata"
	"golang.org/x/build/buildlet"
	"golang.org/x/build/internal/httpdl"
	"golang.org/x/build/pargzip"
)

var (
	haltEntireOS = flag.Bool("halt", true, "halt OS in /halt handler. If false, the buildlet process just ends.")
	rebootOnHalt = flag.Bool("reboot", false, "reboot system in /halt handler.")
	workDir      = flag.String("workdir", "", "Temporary directory to use. The contents of this directory may be deleted at any time. If empty, TempDir is used to create one.")
	listenAddr   = flag.String("listen", "AUTO", "address to listen on. Unused in reverse mode. Warning: this service is inherently insecure and offers no protection of its own. Do not expose this port to the world.")
	reverse      = flag.String("reverse", "", "[deprecated; use --reverse-type instead] if non-empty, go into reverse mode where the buildlet dials the coordinator instead of listening for connections. The value is a comma-separated list of modes, e.g. 'darwin-arm,darwin-amd64-race'")
	reverseType  = flag.String("reverse-type", "", "if non-empty, go into reverse mode where the buildlet dials the coordinator instead of listening for connections. The value is the dashboard/builders.go Hosts map key, naming a HostConfig. This buildlet will receive work for any BuildConfig specifying this named HostConfig.")
	coordinator  = flag.String("coordinator", "localhost:8119", "address of coordinator, in production use farmer.golang.org. Only used in reverse mode.")
	hostname     = flag.String("hostname", "", "hostname to advertise to coordinator for reverse mode; default is actual hostname")
)

// Bump this whenever something notable happens, or when another
// component needs a certain feature. This shows on the coordinator
// per reverse client, and is also accessible via the buildlet
// package's client API (via the Status method).
//
// Notable versions:
//    3: switched to revdial protocol
//    5: reverse dialing uses timeouts+tcp keepalives, pargzip fix
//    7: version bumps while debugging revdial hang (Issue 12816)
//    8: mac screensaver disabled
//   11: move from self-signed cert to LetsEncrypt (Issue 16442)
//   15: ssh support
//   16: make macstadium builders always haltEntireOS
//   17: make macstadium halts use sudo
//   18: set TMPDIR and GOCACHE
const buildletVersion = 18

func defaultListenAddr() string {
	if runtime.GOOS == "darwin" {
		// Darwin will never run on GCE, so let's always
		// listen on a high port (so we don't need to be
		// root).
		return ":5936"
	}
	if !metadata.OnGCE() {
		return "localhost:5936"
	}
	// In production, default to port 80 or 443, depending on
	// whether TLS is configured.
	if metadataValue("tls-cert") != "" {
		return ":443"
	}
	return ":80"
}

// Functionality set non-nil by some platforms:
var (
	osHalt                   func()
	configureSerialLogOutput func()
	setOSRlimit              func() error
)

// If non-empty, the $TMPDIR and $GOCACHE environment variables to use
// for child processes.
var (
	processTmpDirEnv  string
	processGoCacheEnv string
)

func main() {
	switch os.Getenv("GO_BUILDER_ENV") {
	case "macstadium_vm":
		configureMacStadium()
	case "linux-arm-arm5spacemonkey":
		initBaseUnixEnv() // Issue 28041
	}
	onGCE := metadata.OnGCE()
	switch runtime.GOOS {
	case "plan9":
		log.SetOutput(&plan9LogWriter{w: os.Stderr})
	case "linux":
		if onGCE && !inKube {
			if w, err := os.OpenFile("/dev/console", os.O_WRONLY, 0); err == nil {
				log.SetOutput(w)
			}
		}
	case "windows":
		if onGCE {
			configureSerialLogOutput()
		}
	}

	log.Printf("buildlet starting.")
	flag.Parse()

	if *reverse == "solaris-amd64-smartosbuildlet" {
		// These machines were setup without GO_BUILDER_ENV
		// set in their base image, so do init work here after
		// flag parsing instead of at top.
		*rebootOnHalt = true
	}

	// Optimize emphemeral filesystems. Prefer speed over safety,
	// since these VMs only last for the duration of one build.
	switch runtime.GOOS {
	case "openbsd", "freebsd", "netbsd":
		makeBSDFilesystemFast()
	}
	if setOSRlimit != nil {
		err := setOSRlimit()
		if err != nil {
			log.Fatalf("setOSRLimit: %v", err)
		}
		log.Printf("set OS rlimits.")
	}

	if *reverse != "" && *reverseType != "" {
		log.Fatalf("can't specify both --reverse and --reverse-type")
	}
	isReverse := *reverse != "" || *reverseType != ""

	if *listenAddr == "AUTO" && !isReverse {
		v := defaultListenAddr()
		log.Printf("Will listen on %s", v)
		*listenAddr = v
	}

	if !onGCE && !isReverse && !strings.HasPrefix(*listenAddr, "localhost:") {
		log.Printf("** WARNING ***  This server is unsafe and offers no security. Be careful.")
	}
	if onGCE {
		fixMTU()
	}
	if *workDir == "" && setWorkdirToTmpfs != nil {
		setWorkdirToTmpfs()
	}
	if *workDir == "" {
		switch runtime.GOOS {
		case "windows":
			// We want a short path on Windows, due to
			// Windows issues with maximum path lengths.
			*workDir = `C:\workdir`
			if err := os.MkdirAll(*workDir, 0755); err != nil {
				log.Fatalf("error creating workdir: %v", err)
			}
		default:
			wdName := "workdir"
			if *reverseType != "" {
				wdName += "-" + *reverseType
			}
			dir := filepath.Join(os.TempDir(), wdName)
			if err := os.RemoveAll(dir); err != nil { // should be no-op
				log.Fatal(err)
			}
			if err := os.Mkdir(dir, 0755); err != nil {
				log.Fatal(err)
			}
			*workDir = dir
		}
	}

	os.Setenv("WORKDIR", *workDir) // mostly for demos

	if _, err := os.Lstat(*workDir); err != nil {
		log.Fatalf("invalid --workdir %q: %v", *workDir, err)
	}

	// Set up and clean $TMPDIR and $GOCACHE directories.
	if runtime.GOOS != "windows" && runtime.GOOS != "plan9" {
		processTmpDirEnv = filepath.Join(*workDir, "tmp")
		processGoCacheEnv = filepath.Join(*workDir, "gocache")
		removeAllAndMkdir(processTmpDirEnv)
		removeAllAndMkdir(processGoCacheEnv)
	}

	initGorootBootstrap()

	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/debug/goroutines", handleGoroutines)
	http.HandleFunc("/debug/x", handleX)

	var password string
	if !isReverse {
		password = metadataValue("password")
	}
	requireAuth := func(handler func(w http.ResponseWriter, r *http.Request)) http.Handler {
		return requirePasswordHandler{http.HandlerFunc(handler), password}
	}
	http.Handle("/writetgz", requireAuth(handleWriteTGZ))
	http.Handle("/write", requireAuth(handleWrite))
	http.Handle("/exec", requireAuth(handleExec))
	http.Handle("/halt", requireAuth(handleHalt))
	http.Handle("/tgz", requireAuth(handleGetTGZ))
	http.Handle("/removeall", requireAuth(handleRemoveAll))
	http.Handle("/workdir", requireAuth(handleWorkDir))
	http.Handle("/status", requireAuth(handleStatus))
	http.Handle("/ls", requireAuth(handleLs))
	http.Handle("/connect-ssh", requireAuth(handleConnectSSH))

	if !isReverse {
		listenForCoordinator()
	} else {
		if err := dialCoordinator(); err != nil {
			log.Fatalf("Error dialing coordinator: %v", err)
		}
		log.Printf("buildlet reverse mode exiting.")
		os.Exit(0)
	}
}

var inheritedGorootBootstrap string

func initGorootBootstrap() {
	// Remember any GOROOT_BOOTSTRAP to use as a backup in handleExec
	// if $WORKDIR/go1.4 ends up not existing.
	inheritedGorootBootstrap = os.Getenv("GOROOT_BOOTSTRAP")

	// Default if not otherwise configured in dashboard/builders.go:
	os.Setenv("GOROOT_BOOTSTRAP", filepath.Join(*workDir, "go1.4"))

	if runtime.GOOS == "solaris" && runtime.GOARCH == "amd64" {
		gbenv := os.Getenv("GO_BUILDER_ENV")
		if strings.Contains(gbenv, "oracle") {
			// Oracle Solaris; not OpenSolaris-based or
			// Illumos-based.  Do nothing.
			return
		}

		// Assume this is an OpenSolaris-based machine or a
		// SmartOS/Illumos machine before GOOS=="illumos" split.  For
		// these machines, the old Joyent builders need to get the
		// bootstrap and some config fixed.
		os.Setenv("PATH", os.Getenv("PATH")+":/opt/local/bin")
		downloadBootstrapGoroot("/root/go-solaris-amd64-bootstrap", "https://storage.googleapis.com/go-builder-data/gobootstrap-solaris-amd64.tar.gz")
	}
	if runtime.GOOS == "linux" && runtime.GOARCH == "ppc64" {
		downloadBootstrapGoroot("/usr/local/go-bootstrap", "https://storage.googleapis.com/go-builder-data/gobootstrap-linux-ppc64.tar.gz")
	}
}

func downloadBootstrapGoroot(destDir, url string) {
	tarPath := destDir + ".tar.gz"
	origInfo, err := os.Stat(tarPath)
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("Checking for tar existence: %v", err)
	}
	if err := httpdl.Download(tarPath, url); err != nil {
		log.Fatalf("Downloading %s to %s: %v", url, tarPath, err)
	}
	newInfo, err := os.Stat(tarPath)
	if err != nil {
		log.Fatalf("Stat after download: %v", err)
	}
	if os.SameFile(origInfo, newInfo) {
		// The file on disk was unmodified, so we probably untarred it already.
		return
	}
	if err := os.RemoveAll(destDir); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		log.Fatal(err)
	}
	f, err := os.Open(tarPath)
	if err != nil {
		log.Fatalf("Opening after download: %v", err)
	}
	defer f.Close()
	if err := untar(f, destDir); err != nil {
		os.Remove(tarPath)
		os.RemoveAll(destDir)
		log.Fatalf("Untarring %s: %v", url, err)
	}
}

func listenForCoordinator() {
	tlsCert, tlsKey := metadataValue("tls-cert"), metadataValue("tls-key")
	if (tlsCert == "") != (tlsKey == "") {
		log.Fatalf("tls-cert and tls-key must both be supplied, or neither.")
	}

	log.Printf("Listening on %s ...", *listenAddr)
	ln, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", *listenAddr, err)
	}
	ln = tcpKeepAliveListener{ln.(*net.TCPListener)}

	var srv http.Server
	if tlsCert != "" {
		cert, err := tls.X509KeyPair([]byte(tlsCert), []byte(tlsKey))
		if err != nil {
			log.Fatalf("TLS cert error: %v", err)
		}
		tlsConf := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		ln = tls.NewListener(ln, tlsConf)
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(ln)
	}()

	signalChan := make(chan os.Signal, 1)
	if registerSignal != nil {
		registerSignal(signalChan)
	}
	select {
	case sig := <-signalChan:
		log.Printf("received signal %v; shutting down gracefully.", sig)
	case err := <-serveErr:
		log.Fatalf("Serve: %v", err)
	}
	time.AfterFunc(5*time.Second, func() {
		log.Printf("timeout shutting down gracefully; exiting immediately")
		os.Exit(1)
	})
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Printf("Graceful shutdown error: %v; exiting immediately instead", err)
		os.Exit(1)
	}
	log.Printf("graceful shutdown complete.")
	os.Exit(0)
}

// registerSignal if non-nil registers shutdown signals with the provided chan.
var registerSignal func(chan<- os.Signal)

var inKube, _ = strconv.ParseBool(os.Getenv("IN_KUBERNETES"))

// metadataValue returns the GCE metadata instance value for the given key.
// If the metadata is not defined, the returned string is empty.
//
// If not running on GCE, it falls back to using environment variables
// for local development.
func metadataValue(key string) string {
	// The common case (on GCE, but not in Kubernetes):
	if metadata.OnGCE() && !inKube {
		v, err := metadata.InstanceAttributeValue(key)
		if _, notDefined := err.(metadata.NotDefinedError); notDefined {
			return ""
		}
		if err != nil {
			log.Fatalf("metadata.InstanceAttributeValue(%q): %v", key, err)
		}
		return v
	}

	// Else allow use of environment variables to fake
	// metadata keys, for Kubernetes pods or local testing.
	envKey := "META_" + strings.Replace(key, "-", "_", -1)
	v := os.Getenv(envKey)
	// Respect curl-style '@' prefix to mean the rest is a filename.
	if strings.HasPrefix(v, "@") {
		slurp, err := ioutil.ReadFile(v[1:])
		if err != nil {
			log.Fatalf("Error reading file for GCEMETA_%v: %v", key, err)
		}
		return string(slurp)
	}
	if v == "" {
		log.Printf("Warning: not running on GCE, and no %v environment variable defined", envKey)
	}
	return v
}

// tcpKeepAliveListener is a net.Listener that sets TCP keep-alive
// timeouts on accepted connections.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

func fixMTU_freebsd() error { return fixMTU_ifconfig("vtnet0") }
func fixMTU_openbsd() error { return fixMTU_ifconfig("vio0") }
func fixMTU_ifconfig(iface string) error {
	out, err := exec.Command("/sbin/ifconfig", iface, "mtu", "1460").CombinedOutput()
	if err != nil {
		return fmt.Errorf("/sbin/ifconfig %s mtu 1460: %v, %s", iface, err, out)
	}
	return nil
}

func fixMTU_plan9() error {
	f, err := os.OpenFile("/net/ipifc/0/ctl", os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(f, "mtu 1460\n"); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func fixMTU() {
	fn, ok := map[string]func() error{
		"openbsd": fixMTU_openbsd,
		"freebsd": fixMTU_freebsd,
		"plan9":   fixMTU_plan9,
	}[runtime.GOOS]
	if ok {
		if err := fn(); err != nil {
			log.Printf("Failed to set MTU: %v", err)
		} else {
			log.Printf("Adjusted MTU.")
		}
	}
}

// flushWriter is an io.Writer that Flushes after each Write if the
// underlying Writer implements http.Flusher.
type flushWriter struct {
	rw http.ResponseWriter
}

func (fw flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.rw.Write(p)
	if f, ok := fw.rw.(http.Flusher); ok {
		f.Flush()
	}
	return
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	fmt.Fprintf(w, "buildlet running on %s-%s\n", runtime.GOOS, runtime.GOARCH)
}

// unauthenticated /debug/goroutines handler
func handleGoroutines(w http.ResponseWriter, r *http.Request) {
	log.Printf("Dumping goroutines.")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	buf := make([]byte, 2<<20)
	buf = buf[:runtime.Stack(buf, true)]
	w.Write(buf)
	log.Printf("Dumped goroutines.")
}

// unauthenticated /debug/x handler, to test MTU settings.
func handleX(w http.ResponseWriter, r *http.Request) {
	n, _ := strconv.Atoi(r.FormValue("n"))
	if n > 1<<20 {
		n = 1 << 20
	}
	log.Printf("Dumping %d X.", n)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = 'X'
	}
	w.Write(buf)
	log.Printf("Dumped X.")
}

// This is a remote code execution daemon, so security is kinda pointless, but:
func validRelativeDir(dir string) bool {
	if strings.Contains(dir, `\`) || path.IsAbs(dir) {
		return false
	}
	dir = path.Clean(dir)
	if strings.HasPrefix(dir, "../") || strings.HasSuffix(dir, "/..") || dir == ".." {
		return false
	}
	return true
}

func handleGetTGZ(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "requires GET method", http.StatusBadRequest)
		return
	}
	dir := r.FormValue("dir")
	if !validRelativeDir(dir) {
		http.Error(w, "bogus dir", http.StatusBadRequest)
		return
	}
	var zw io.WriteCloser
	if r.FormValue("pargzip") == "0" {
		zw = gzip.NewWriter(w)
	} else {
		zw = pargzip.NewWriter(w)
	}
	tw := tar.NewWriter(zw)
	base := filepath.Join(*workDir, filepath.FromSlash(dir))
	err := filepath.Walk(base, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(filepath.ToSlash(strings.TrimPrefix(path, base)), "/")
		var linkName string
		if fi.Mode()&os.ModeSymlink != 0 {
			linkName, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		th, err := tar.FileInfoHeader(fi, linkName)
		if err != nil {
			return err
		}
		th.Name = rel
		if fi.IsDir() && !strings.HasSuffix(th.Name, "/") {
			th.Name += "/"
		}
		if th.Name == "/" {
			return nil
		}
		if err := tw.WriteHeader(th); err != nil {
			return err
		}
		if fi.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Walk error: %v", err)
		panic(http.ErrAbortHandler)
	}
	tw.Close()
	zw.Close()
}

func handleWriteTGZ(w http.ResponseWriter, r *http.Request) {
	urlParam, _ := url.ParseQuery(r.URL.RawQuery)
	baseDir := *workDir
	if dir := urlParam.Get("dir"); dir != "" {
		if !validRelativeDir(dir) {
			log.Printf("writetgz: bogus dir %q", dir)
			http.Error(w, "bogus dir", http.StatusBadRequest)
			return
		}
		dir = filepath.FromSlash(dir)
		baseDir = filepath.Join(baseDir, dir)

		// Special case: if the directory is "go1.4" and it already exists, do nothing.
		// This lets clients do a blind write to it and not do extra work.
		if r.Method == "POST" && dir == "go1.4" {
			if fi, err := os.Stat(baseDir); err == nil && fi.IsDir() {
				log.Printf("writetgz: skipping URL puttar to go1.4 dir; already exists")
				io.WriteString(w, "SKIP")
				return
			}
		}

		if err := os.MkdirAll(baseDir, 0755); err != nil {
			log.Printf("writetgz: %v", err)
			http.Error(w, "mkdir of base: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	var tgz io.Reader
	var urlStr string
	switch r.Method {
	case "PUT":
		tgz = r.Body
		log.Printf("writetgz: untarring Request.Body into %s", baseDir)
	case "POST":
		urlStr = r.FormValue("url")
		if urlStr == "" {
			log.Printf("writetgz: missing url POST param")
			http.Error(w, "missing url POST param", http.StatusBadRequest)
			return
		}
		t0 := time.Now()
		res, err := http.Get(urlStr)
		if err != nil {
			log.Printf("writetgz: failed to fetch tgz URL %s: %v", urlStr, err)
			http.Error(w, fmt.Sprintf("fetching URL %s: %v", urlStr, err), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			log.Printf("writetgz: failed to fetch tgz URL %s: status=%v", urlStr, res.Status)
			http.Error(w, fmt.Sprintf("writetgz: fetching provided URL %q: %s", urlStr, res.Status), http.StatusInternalServerError)
			return
		}
		tgz = res.Body
		log.Printf("writetgz: untarring %s (got headers in %v) into %s", urlStr, time.Since(t0), baseDir)
	default:
		log.Printf("writetgz: invalid method %q", r.Method)
		http.Error(w, "requires PUT or POST method", http.StatusBadRequest)
		return
	}

	err := untar(tgz, baseDir)
	if err != nil {
		status := http.StatusInternalServerError
		if he, ok := err.(httpStatuser); ok {
			status = he.httpStatus()
		}
		http.Error(w, err.Error(), status)
		return
	}
	io.WriteString(w, "OK")
}

func handleWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		http.Error(w, "requires POST method", http.StatusBadRequest)
		return
	}

	param, _ := url.ParseQuery(r.URL.RawQuery)

	path := param.Get("path")
	if path == "" || !validRelPath(path) {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	path = filepath.FromSlash(path)
	path = filepath.Join(*workDir, path)

	modeInt, err := strconv.ParseInt(param.Get("mode"), 10, 64)
	mode := os.FileMode(modeInt)
	if err != nil || !mode.IsRegular() {
		http.Error(w, "bad mode", http.StatusBadRequest)
		return
	}

	// Make the directory if it doesn't exist.
	// TODO(adg): support dirmode parameter?
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := writeFile(r.Body, path, mode); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	io.WriteString(w, "OK")
}

func writeFile(r io.Reader, path string, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	// Try to set the mode again, in case the file already existed.
	if runtime.GOOS != "windows" {
		if err := f.Chmod(mode); err != nil {
			f.Close()
			return err
		}
	}
	return f.Close()
}

// untar reads the gzip-compressed tar file from r and writes it into dir.
func untar(r io.Reader, dir string) (err error) {
	t0 := time.Now()
	nFiles := 0
	madeDir := map[string]bool{}
	defer func() {
		td := time.Since(t0)
		if err == nil {
			log.Printf("extracted tarball into %s: %d files, %d dirs (%v)", dir, nFiles, len(madeDir), td)
		} else {
			log.Printf("error extracting tarball into %s after %d files, %d dirs, %v: %v", dir, nFiles, len(madeDir), td, err)
		}
	}()
	zr, err := gzip.NewReader(r)
	if err != nil {
		return badRequest("requires gzip-compressed body: " + err.Error())
	}
	tr := tar.NewReader(zr)
	loggedChtimesError := false
	for {
		f, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("tar reading error: %v", err)
			return badRequest("tar error: " + err.Error())
		}
		if f.Typeflag == tar.TypeXGlobalHeader {
			// golang.org/issue/22748: git archive exports
			// a global header ('g') which after Go 1.9
			// (for a bit?) contained an empty filename.
			// Ignore it.
			continue
		}
		if !validRelPath(f.Name) {
			return badRequest(fmt.Sprintf("tar file contained invalid name %q", f.Name))
		}
		rel := filepath.FromSlash(f.Name)
		abs := filepath.Join(dir, rel)

		fi := f.FileInfo()
		mode := fi.Mode()
		switch {
		case mode.IsRegular():
			// Make the directory. This is redundant because it should
			// already be made by a directory entry in the tar
			// beforehand. Thus, don't check for errors; the next
			// write will fail with the same error.
			dir := filepath.Dir(abs)
			if !madeDir[dir] {
				if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
					return err
				}
				madeDir[dir] = true
			}
			wf, err := os.OpenFile(abs, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode.Perm())
			if err != nil {
				return err
			}
			n, err := io.Copy(wf, tr)
			if closeErr := wf.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
			if err != nil {
				return fmt.Errorf("error writing to %s: %v", abs, err)
			}
			if n != f.Size {
				return fmt.Errorf("only wrote %d bytes to %s; expected %d", n, abs, f.Size)
			}
			modTime := f.ModTime
			if modTime.After(t0) {
				// Clamp modtimes at system time. See
				// golang.org/issue/19062 when clock on
				// buildlet was behind the gitmirror server
				// doing the git-archive.
				modTime = t0
			}
			if !modTime.IsZero() {
				if err := os.Chtimes(abs, modTime, modTime); err != nil && !loggedChtimesError {
					// benign error. Gerrit doesn't even set the
					// modtime in these, and we don't end up relying
					// on it anywhere (the gomote push command relies
					// on digests only), so this is a little pointless
					// for now.
					log.Printf("error changing modtime: %v (further Chtimes errors suppressed)", err)
					loggedChtimesError = true // once is enough
				}
			}
			nFiles++
		case mode.IsDir():
			if err := os.MkdirAll(abs, 0755); err != nil {
				return err
			}
			madeDir[abs] = true
		default:
			return badRequest(fmt.Sprintf("tar file entry %s contained unsupported file type %v", f.Name, mode))
		}
	}
	return nil
}

// Process-State is an HTTP Trailer set in the /exec handler to "ok"
// on success, or os.ProcessState.String() on failure.
const hdrProcessState = "Process-State"

func handleExec(w http.ResponseWriter, r *http.Request) {
	cn := w.(http.CloseNotifier)
	clientGone := cn.CloseNotify()
	handlerDone := make(chan bool)
	defer close(handlerDone)

	if r.Method != "POST" {
		http.Error(w, "requires POST method", http.StatusBadRequest)
		return
	}
	if r.ProtoMajor*10+r.ProtoMinor < 11 {
		// We need trailers, only available in HTTP/1.1 or HTTP/2.
		http.Error(w, "HTTP/1.1 or higher required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Trailer", hdrProcessState) // declare it so we can set it

	cmdPath := r.FormValue("cmd") // required
	absCmd := cmdPath
	dir := r.FormValue("dir") // optional
	sysMode := r.FormValue("mode") == "sys"
	debug, _ := strconv.ParseBool(r.FormValue("debug"))

	if sysMode {
		if cmdPath == "" {
			http.Error(w, "requires 'cmd' parameter", http.StatusBadRequest)
			return
		}
		if dir == "" {
			dir = *workDir
		} else {
			dir = filepath.FromSlash(dir)
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(*workDir, dir)
			}
		}
	} else {
		if !validRelPath(cmdPath) {
			http.Error(w, "requires 'cmd' parameter", http.StatusBadRequest)
			return
		}
		absCmd = filepath.Join(*workDir, filepath.FromSlash(cmdPath))
		if dir == "" {
			dir = filepath.Dir(absCmd)
		} else {
			if !validRelPath(dir) {
				http.Error(w, "bogus 'dir' parameter", http.StatusBadRequest)
				return
			}
			dir = filepath.Join(*workDir, filepath.FromSlash(dir))
		}
	}

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	goarch := "amd64" // unless we find otherwise
	for _, pair := range r.PostForm["env"] {
		if hasPrefixFold(pair, "GOARCH=") {
			goarch = pair[len("GOARCH="):]
		}
	}

	env := append(baseEnv(goarch), r.PostForm["env"]...)

	if v := processTmpDirEnv; v != "" {
		env = append(env, "TMPDIR="+v)
	}
	if v := processGoCacheEnv; v != "" {
		env = append(env, "GOCACHE="+v)
	}

	// Prefer buildlet process's inherited GOROOT_BOOTSTRAP if
	// there was one and the one we're about to use doesn't exist.
	if v := getEnv(env, "GOROOT_BOOTSTRAP"); v != "" && inheritedGorootBootstrap != "" && pathNotExist(v) {
		env = append(env, "GOROOT_BOOTSTRAP="+inheritedGorootBootstrap)
	}
	env = setPathEnv(env, r.PostForm["path"], *workDir)

	cmd := exec.Command(absCmd, r.PostForm["cmdArg"]...)
	cmd.Dir = dir
	cmdOutput := flushWriter{w}
	cmd.Stdout = cmdOutput
	cmd.Stderr = cmdOutput
	cmd.Env = env

	log.Printf("[%p] Running %s with args %q and env %q in dir %s",
		cmd, cmd.Path, cmd.Args, cmd.Env, cmd.Dir)

	if debug {
		fmt.Fprintf(cmdOutput, ":: Running %s with args %q and env %q in dir %s\n\n",
			cmd.Path, cmd.Args, cmd.Env, cmd.Dir)
	}

	t0 := time.Now()
	err := cmd.Start()
	if err == nil {
		go func() {
			select {
			case <-clientGone:
				err := killProcessTree(cmd.Process)
				if err != nil {
					log.Printf("Kill failed: %v", err)
				}
			case <-handlerDone:
				return
			}
		}()
		err = cmd.Wait()
	}
	state := "ok"
	if err != nil {
		if ps := cmd.ProcessState; ps != nil {
			state = ps.String()
		} else {
			state = err.Error()
		}
	}
	w.Header().Set(hdrProcessState, state)
	log.Printf("[%p] Run = %s, after %v", cmd, state, time.Since(t0))
}

// pathNotExist reports whether path does not exist.
func pathNotExist(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

func getEnv(env []string, key string) string {
	for _, kv := range env {
		if len(kv) <= len(key) || kv[len(key)] != '=' {
			continue
		}
		if runtime.GOOS == "windows" {
			// Case insensitive.
			if strings.EqualFold(kv[:len(key)], key) {
				return kv[len(key)+1:]
			}
		} else {
			// Case sensitive.
			if kv[:len(key)] == key {
				return kv[len(key)+1:]
			}
		}
	}
	return ""
}

// setPathEnv returns a copy of the provided environment with any existing
// PATH variables replaced by the user-provided path.
// These substitutions are applied to user-supplied path elements:
//   - the string "$PATH" expands to the original PATH elements
//   - the substring "$WORKDIR" expands to the provided workDir
// A path of just ["$EMPTY"] removes the PATH variable from the environment.
func setPathEnv(env, path []string, workDir string) []string {
	if len(path) == 0 {
		return env
	}

	var (
		pathIdx  = -1
		pathOrig = ""
	)

	for i, s := range env {
		if isPathEnvPair(s) {
			pathIdx = i
			pathOrig = s[len("PaTh="):] // in whatever case
			break
		}
	}
	if len(path) == 1 && path[0] == "$EMPTY" {
		// Remove existing path variable if it exists.
		if pathIdx >= 0 {
			env = append(env[:pathIdx], env[pathIdx+1:]...)
		}
		return env
	}

	// Apply substitions to a copy of the path argument.
	path = append([]string{}, path...)
	for i, s := range path {
		if s == "$PATH" {
			path[i] = pathOrig // ok if empty
		} else {
			path[i] = strings.Replace(s, "$WORKDIR", workDir, -1)
		}
	}

	// Put the new PATH in env.
	env = append([]string{}, env...)
	pathEnv := pathEnvVar() + "=" + strings.Join(path, pathSeparator())
	if pathIdx >= 0 {
		env[pathIdx] = pathEnv
	} else {
		env = append(env, pathEnv)
	}

	return env
}

// isPathEnvPair reports whether the key=value pair s represents
// the operating system's path variable.
func isPathEnvPair(s string) bool {
	// On Unix it's PATH.
	// On Plan 9 it's path.
	// On Windows it's pAtH case-insensitive.
	if runtime.GOOS == "windows" {
		return len(s) >= 5 && strings.EqualFold(s[:5], "PATH=")
	}
	if runtime.GOOS == "plan9" {
		return strings.HasPrefix(s, "path=")
	}
	return strings.HasPrefix(s, "PATH=")
}

// On Unix it's PATH.
// On Plan 9 it's path.
// On Windows it's pAtH case-insensitive.
func pathEnvVar() string {
	if runtime.GOOS == "plan9" {
		return "path"
	}
	return "PATH"
}

func pathSeparator() string {
	if runtime.GOOS == "plan9" {
		return "\x00"
	} else {
		return string(filepath.ListSeparator)
	}
}

func baseEnv(goarch string) []string {
	if runtime.GOOS == "windows" {
		return windowsBaseEnv(goarch)
	}
	return os.Environ()
}

func windowsBaseEnv(goarch string) (e []string) {
	e = append(e, "GOBUILDEXIT=1") // exit all.bat with completion status

	is64 := goarch != "386"
	for _, pair := range os.Environ() {
		const pathEq = "PATH="
		if hasPrefixFold(pair, pathEq) {
			e = append(e, "PATH="+windowsPath(pair[len(pathEq):], is64))
		} else {
			e = append(e, pair)
		}
	}
	return e
}

// hasPrefixFold is a case-insensitive strings.HasPrefix.
func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// windowsPath cleans the windows %PATH% environment.
// is64Bit is whether this is a windows-amd64-* builder.
// The PATH is assumed to be that of the image described in env/windows/README.
func windowsPath(old string, is64Bit bool) string {
	vv := filepath.SplitList(old)
	newPath := make([]string, 0, len(vv))

	// for windows-buildlet-v2 images
	for _, v := range vv {
		// The base VM image has both the 32-bit and 64-bit gcc installed.
		// They're both in the environment, so scrub the one
		// we don't want (TDM-GCC-64 or TDM-GCC-32).
		if strings.Contains(v, "TDM-GCC-") {
			gcc64 := strings.Contains(v, "TDM-GCC-64")
			if is64Bit != gcc64 {
				continue
			}
		}
		newPath = append(newPath, v)
	}

	// for windows-amd64-* images
	if is64Bit {
		newPath = append(newPath, `C:\godep\gcc64\bin`)
	} else {
		newPath = append(newPath, `C:\godep\gcc32\bin`)
	}

	return strings.Join(newPath, string(filepath.ListSeparator))
}

func handleHalt(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "requires POST method", http.StatusBadRequest)
		return
	}

	// Do the halt in 1 second, to give the HTTP response time to
	// complete.
	//
	// TODO(bradfitz): maybe prevent any (unlikely) future HTTP
	// requests from doing anything from this point on in the
	// remaining second.
	log.Printf("Halting in 1 second.")
	time.AfterFunc(1*time.Second, doHalt)
}

func doHalt() {
	if *rebootOnHalt {
		if err := exec.Command("reboot").Run(); err != nil {
			log.Printf("Error running reboot: %v", err)
		}
		os.Exit(0)
	}
	if !*haltEntireOS {
		log.Printf("Ending buildlet process due to halt.")
		os.Exit(0)
		return
	}
	log.Printf("Halting machine.")
	time.AfterFunc(5*time.Second, func() { os.Exit(0) })
	if osHalt != nil {
		// TODO: Windows: http://msdn.microsoft.com/en-us/library/windows/desktop/aa376868%28v=vs.85%29.aspx
		osHalt()
		os.Exit(0)
	}
	// Backup mechanism, if exec hangs for any reason:
	var err error
	switch runtime.GOOS {
	case "openbsd":
		// Quick, no fs flush, and power down:
		err = exec.Command("halt", "-q", "-n", "-p").Run()
	case "freebsd":
		// Power off (-p), via halt (-o), now.
		err = exec.Command("shutdown", "-p", "-o", "now").Run()
	case "linux":
		// Don't sync (-n), force without shutdown (-f), and power off (-p).
		err = exec.Command("/bin/halt", "-n", "-f", "-p").Run()
	case "plan9":
		err = exec.Command("fshalt").Run()
	case "darwin":
		if os.Getenv("GO_BUILDER_ENV") == "macstadium_vm" {
			// Fast, sloppy, unsafe, because we're never reusing this VM again.
			err = exec.Command("/usr/bin/sudo", "/sbin/halt", "-n", "-q", "-l").Run()
		} else {
			err = errors.New("not respecting -halt flag on macOS in unknown environment")
		}
	default:
		err = errors.New("No system-specific halt command run; will just end buildlet process.")
	}
	log.Printf("Shutdown: %v", err)
	log.Printf("Ending buildlet process post-halt")
	os.Exit(0)
}

func handleRemoveAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "requires POST method", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	paths := r.Form["path"]
	if len(paths) == 0 {
		http.Error(w, "requires 'path' parameter", http.StatusBadRequest)
		return
	}
	for _, p := range paths {
		if !validRelPath(p) {
			http.Error(w, fmt.Sprintf("bad 'path' parameter: %q", p), http.StatusBadRequest)
			return
		}
	}
	for _, p := range paths {
		log.Printf("Removing %s", p)
		fullDir := filepath.Join(*workDir, filepath.FromSlash(p))
		err := os.RemoveAll(fullDir)
		if p == "." && err != nil {
			// If workDir is a mountpoint and/or contains a binary
			// using it, we can get a "Device or resource busy" error.
			// See if it's now empty and ignore the error.
			if f, oerr := os.Open(*workDir); oerr == nil {
				if all, derr := f.Readdirnames(-1); derr == nil && len(all) == 0 {
					log.Printf("Ignoring fail of RemoveAll(.)")
					err = nil
				} else {
					log.Printf("Readdir = %q, %v", all, derr)
				}
				f.Close()
			} else {
				log.Printf("Failed to open workdir: %v", oerr)
			}
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	// If we nuked the work directory (or tmp or gocache), recreate them.
	for _, dir := range []string{*workDir, processTmpDirEnv, processGoCacheEnv} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func handleWorkDir(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "requires GET method", http.StatusBadRequest)
		return
	}
	fmt.Fprint(w, *workDir)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "requires GET method", http.StatusBadRequest)
		return
	}
	status := buildlet.Status{
		Version: buildletVersion,
	}
	b, err := json.Marshal(status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(b)
}

func handleLs(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "requires GET method", http.StatusBadRequest)
		return
	}
	dir := r.FormValue("dir")
	recursive, _ := strconv.ParseBool(r.FormValue("recursive"))
	digest, _ := strconv.ParseBool(r.FormValue("digest"))
	skip := r.Form["skip"] // '/'-separated relative dirs

	if !validRelativeDir(dir) {
		http.Error(w, "bogus dir", http.StatusBadRequest)
		return
	}
	base := filepath.Join(*workDir, filepath.FromSlash(dir))
	anyOutput := false
	err := filepath.Walk(base, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(filepath.ToSlash(strings.TrimPrefix(path, base)), "/")
		if rel == "" && fi.IsDir() {
			return nil
		}
		if fi.IsDir() {
			for _, v := range skip {
				if rel == v {
					return filepath.SkipDir
				}
			}
		}
		anyOutput = true
		fmt.Fprintf(w, "%s\t%s", fi.Mode(), rel)
		if fi.Mode().IsRegular() {
			fmt.Fprintf(w, "\t%d\t%s", fi.Size(), fi.ModTime().UTC().Format(time.RFC3339))
			if digest {
				if sha1, err := fileSHA1(path); err != nil {
					return err
				} else {
					io.WriteString(w, "\t"+sha1)
				}
			}
		} else if fi.Mode().IsDir() {
			io.WriteString(w, "/")
		}
		io.WriteString(w, "\n")
		if fi.IsDir() && !recursive {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		log.Printf("Walk error: %v", err)
		if anyOutput {
			// Decent way to signal failure to the caller, since it'll break
			// the chunked response, rather than have a valid EOF.
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
			return
		}
		http.Error(w, "Walk error: "+err.Error(), 500)
		return
	}
}

func handleConnectSSH(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "requires POST method", http.StatusBadRequest)
		return
	}
	if r.ContentLength != 0 {
		http.Error(w, "requires zero Content-Length", http.StatusBadRequest)
		return
	}
	sshUser := r.Header.Get("X-Go-Ssh-User")
	authKey := r.Header.Get("X-Go-Authorized-Key")
	if sshUser != "" && authKey != "" {
		if err := appendSSHAuthorizedKey(sshUser, authKey); err != nil {
			http.Error(w, "adding ssh authorized key: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	sshConn, err := net.Dial("tcp", "localhost:"+sshPort())
	if err != nil {
		sshServerOnce.Do(startSSHServer)
		sshConn, err = net.Dial("tcp", "localhost:"+sshPort())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
	}
	defer sshConn.Close()
	hj, ok := w.(http.Hijacker)
	if !ok {
		log.Printf("conn can't hijack for ssh proxy; HTTP/2 enabled by default?")
		http.Error(w, "conn can't hijack", http.StatusInternalServerError)
		return
	}
	conn, _, err := hj.Hijack()
	if err != nil {
		log.Printf("ssh hijack error: %v", err)
		http.Error(w, "ssh hijack error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()
	fmt.Fprintf(conn, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: ssh\r\nConnection: Upgrade\r\n\r\n")
	errc := make(chan error, 1)
	go func() {
		_, err := io.Copy(sshConn, conn)
		errc <- err
	}()
	go func() {
		_, err := io.Copy(conn, sshConn)
		errc <- err
	}()
	<-errc
}

// sshPort returns the port to use for the local SSH server.
func sshPort() string {
	// runningInCOS is whether we're running under GCE's Container-Optimized OS (COS).
	const runningInCOS = runtime.GOOS == "linux" && runtime.GOARCH == "amd64"

	if runningInCOS {
		// If running in COS, we can't use port 22, as the system's sshd is already using it.
		// Our container runs in the system network namespace, not isolated as is typical
		// in Docker or Kubernetes. So use another high port. See https://golang.org/issue/26969.
		return "2200"
	}
	return "22"
}

var sshServerOnce sync.Once

// startSSHServer starts an SSH server.
func startSSHServer() {
	if inLinuxContainer() {
		startSSHServerLinux()
		return
	}
	if runtime.GOOS == "netbsd" {
		startSSHServerNetBSD()
		return
	}

	log.Printf("start ssh server: don't know how to start SSH server on this host type")
}

// inLinuxContainer reports whether it looks like we're on Linux running inside a container.
func inLinuxContainer() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if numProcs() >= 4 {
		// There should 1 process running (this buildlet
		// binary) if we're in Docker. Maybe 2 if something
		// else is happening. But if there are 4 or more,
		// we'll be paranoid and assuming we're running on a
		// user or host system and don't want to start an ssh
		// server.
		return false
	}
	// TODO: use a more explicit env variable or on-disk signal
	// that we're in a Go buildlet Docker image. But for now, this
	// seems to be consistently true:
	fi, err := os.Stat("/usr/local/bin/stage0")
	return err == nil && fi.Mode().IsRegular()
}

// startSSHServerLinux starts an SSH server on a Linux system.
func startSSHServerLinux() {
	log.Printf("start ssh server for linux")

	// First, create the privsep directory, otherwise we get a successful cmd.Start,
	// but this error message and then an exit:
	//    Missing privilege separation directory: /var/run/sshd
	if err := os.MkdirAll("/var/run/sshd", 0700); err != nil {
		log.Printf("creating /var/run/sshd: %v", err)
		return
	}

	// The scaleway Docker images don't have ssh host keys in
	// their image, at least as of 2017-07-23. So make them first.
	// These are the types sshd -D complains about currently.
	if runtime.GOARCH == "arm" {
		for _, keyType := range []string{"rsa", "dsa", "ed25519", "ecdsa"} {
			file := "/etc/ssh/ssh_host_" + keyType + "_key"
			if _, err := os.Stat(file); err == nil {
				continue
			}
			out, err := exec.Command("/usr/bin/ssh-keygen", "-f", file, "-N", "", "-t", keyType).CombinedOutput()
			log.Printf("ssh-keygen of type %s: err=%v, %s\n", keyType, err, out)
		}
	}

	cmd := exec.Command("/usr/sbin/sshd", "-D", "-p", sshPort())
	err := cmd.Start()
	if err != nil {
		log.Printf("starting sshd: %v", err)
		return
	}
	log.Printf("sshd started.")
	waitLocalSSH()
}

func startSSHServerNetBSD() {
	cmd := exec.Command("/etc/rc.d/sshd", "start")
	err := cmd.Start()
	if err != nil {
		log.Printf("starting sshd: %v", err)
		return
	}
	log.Printf("sshd started.")
	waitLocalSSH()
}

// waitLocalSSH waits for sshd to start accepting connections.
func waitLocalSSH() {
	for i := 0; i < 40; i++ {
		time.Sleep(10 * time.Millisecond * time.Duration(i+1))
		c, err := net.Dial("tcp", "localhost:"+sshPort())
		if err == nil {
			c.Close()
			log.Printf("sshd connected.")
			return
		}
	}
	log.Printf("timeout waiting for sshd to come up")
}

func numProcs() int {
	n := 0
	fis, _ := ioutil.ReadDir("/proc")
	for _, fi := range fis {
		if _, err := strconv.Atoi(fi.Name()); err == nil {
			n++
		}
	}
	return n
}

func fileSHA1(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	s1 := sha1.New()
	if _, err := io.Copy(s1, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", s1.Sum(nil)), nil
}

func validRelPath(p string) bool {
	if p == "" || strings.Contains(p, `\`) || strings.HasPrefix(p, "/") || strings.Contains(p, "../") {
		return false
	}
	return true
}

type httpStatuser interface {
	error
	httpStatus() int
}

type httpError struct {
	statusCode int
	msg        string
}

func (he httpError) Error() string   { return he.msg }
func (he httpError) httpStatus() int { return he.statusCode }

func badRequest(msg string) error {
	return httpError{http.StatusBadRequest, msg}
}

// requirePassword is an http.Handler auth wrapper that enforces a
// HTTP Basic password. The username is ignored.
type requirePasswordHandler struct {
	h        http.Handler
	password string // empty means no password
}

func (h requirePasswordHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, gotPass, _ := r.BasicAuth()
	if h.password != "" && h.password != gotPass {
		http.Error(w, "invalid password", http.StatusForbidden)
		return
	}
	h.h.ServeHTTP(w, r)
}

// plan9LogWriter truncates log writes to 128 bytes,
// to work around some Plan 9 and/or GCE serial port bug.
type plan9LogWriter struct {
	w   io.Writer
	buf []byte
}

func (pw *plan9LogWriter) Write(p []byte) (n int, err error) {
	const max = 128 - len("\n\x00")
	if len(p) < max {
		return pw.w.Write(p)
	}
	if pw.buf == nil {
		pw.buf = make([]byte, max+1)
	}
	n = copy(pw.buf[:max], p)
	pw.buf[n] = '\n'
	return pw.w.Write(pw.buf[:n+1])
}

func requireTrailerSupport() {
	// Depend on a symbol that was added after HTTP Trailer support was
	// implemented (4b96409 Dec 29 2014) so that this function will fail
	// to compile without Trailer support.
	// bufio.Reader.Discard was added by ee2ecc4 Jan 7 2015.
	var r bufio.Reader
	_ = r.Discard
}

var killProcessTree = killProcessTreeUnix

func killProcessTreeUnix(p *os.Process) error {
	return p.Kill()
}

// configureMacStadium configures the buildlet flags for use on a Mac
// VM running on MacStadium under VMWare.
func configureMacStadium() {
	*haltEntireOS = true

	// TODO: setup RAM disk for tmp and set *workDir

	disableMacScreensaver()

	version, err := exec.Command("sw_vers", "-productVersion").Output()
	if err != nil {
		log.Fatalf("failed to find sw_vers -productVersion: %v", err)
	}
	majorMinor := regexp.MustCompile(`^(\d+)\.(\d+)`)
	m := majorMinor.FindStringSubmatch(string(version))
	if m == nil {
		log.Fatalf("unsupported sw_vers version %q", version)
	}
	major, minor := m[1], m[2] // "10", "12"
	*reverse = "darwin-amd64-" + major + "_" + minor
	*coordinator = "farmer.golang.org:443"

	// guestName is set by cmd/makemac to something like
	// "mac_10_10_host01b" or "mac_10_12_host01a", which encodes
	// three things: the mac OS version of the guest VM, which
	// physical machine it's on (1 to 10, currently) and which of
	// two possible VMs on that host is running (a or b). For
	// monitoring purposes, we want stable hostnames and don't
	// care which OS version is currently running (which changes
	// constantly), so normalize these to only have the host
	// number and side (a or b), without the OS version. The
	// buildlet will report the OS version to the coordinator
	// anyway. We could in theory do this normalization in the
	// coordinator, but we don't want to put buildlet-specific
	// knowledge there, and this file already contains a bunch of
	// buildlet host-specific configuration, so normalize it here.
	guestName := vmwareGetInfo("guestinfo.name") // "mac_10_12_host01a"
	hostPos := strings.Index(guestName, "_host")
	if hostPos == -1 {
		// Assume cmd/makemac changed its conventions.
		// Maybe all this normalization belongs there anyway,
		// but normalizing here is a safer first step.
		*hostname = guestName
	} else {
		*hostname = "macstadium" + guestName[hostPos:] // "macstadium_host01a"
	}
}

func disableMacScreensaver() {
	err := exec.Command("defaults", "-currentHost", "write", "com.apple.screensaver", "idleTime", "0").Run()
	if err != nil {
		log.Printf("disabling screensaver: %v", err)
	}
}

func vmwareGetInfo(key string) string {
	cmd := exec.Command("/Library/Application Support/VMware Tools/vmware-tools-daemon",
		"--cmd",
		"info-get "+key)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if strings.Contains(stderr.String(), "No value found") {
			return ""
		}
		log.Fatalf("Error running vmware-tools-daemon --cmd 'info-get %s': %v, %s\n%s", key, err, stderr.Bytes(), stdout.Bytes())
	}
	return strings.TrimSpace(stdout.String())
}

func makeBSDFilesystemFast() {
	if !metadata.OnGCE() {
		log.Printf("Not on GCE; not remounting root filesystem.")
		return
	}
	btype, err := metadata.InstanceAttributeValue("buildlet-host-type")
	if _, ok := err.(metadata.NotDefinedError); ok && len(btype) == 0 {
		log.Printf("Not remounting root filesystem due to missing buildlet-host-type metadata.")
		return
	}
	if err != nil {
		log.Printf("Not remounting root filesystem due to failure getting builder type instance metadata: %v", err)
		return
	}
	// Tested on OpenBSD, FreeBSD, and NetBSD:
	out, err := exec.Command("/sbin/mount", "-u", "-o", "async,noatime", "/").CombinedOutput()
	if err != nil {
		log.Printf("Warning: failed to remount %s root filesystem with async,noatime: %v, %s", runtime.GOOS, err, out)
		return
	}
	log.Printf("Remounted / with async,noatime.")
}

func appendSSHAuthorizedKey(sshUser, authKey string) error {
	var homeRoot string
	switch runtime.GOOS {
	case "darwin":
		homeRoot = "/Users"
	case "plan9":
		return fmt.Errorf("ssh not supported on %v", runtime.GOOS)
	case "windows":
		homeRoot = `C:\Users`
	default:
		homeRoot = "/home"
		if runtime.GOOS == "freebsd" {
			if fi, err := os.Stat("/usr/home/" + sshUser); err == nil && fi.IsDir() {
				homeRoot = "/usr/home"
			}
		}
		if sshUser == "root" {
			homeRoot = "/"
		}
	}
	sshDir := filepath.Join(homeRoot, sshUser, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(sshDir, 0700); err != nil {
		return err
	}
	authFile := filepath.Join(sshDir, "authorized_keys")
	exist, err := ioutil.ReadFile(authFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(exist), authKey) {
		return nil
	}
	f, err := os.OpenFile(authFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "%s\n", authKey); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if runtime.GOOS == "freebsd" {
		exec.Command("/usr/sbin/chown", "-R", sshUser, sshDir).Run()
	}
	if runtime.GOOS == "windows" {
		if res, err := exec.Command("icacls.exe", authFile, "/grant", `NT SERVICE\sshd:(R)`).CombinedOutput(); err != nil {
			return fmt.Errorf("setting permissions on authorized_keys with: %v\n%s.", err, res)
		}
	}
	return nil
}

// setWorkdirToTmpfs sets the *workDir (--workdir) flag to /workdir
// if the flag is empty and /workdir is a tmpfs mount, as it is on the various
// hosts that use rundockerbuildlet.
//
// It is set non-nil on operating systems where the functionality is
// needed & available. Currently we only use it on Linux.
var setWorkdirToTmpfs func()

func initBaseUnixEnv() {
	if os.Getenv("USER") == "" {
		os.Setenv("USER", "root")
	}
	if os.Getenv("HOME") == "" {
		os.Setenv("HOME", "/root")
	}
}

// removeAllAndMkdir calls os.RemoveAll and then os.Mkdir on the given
// dir, failing the process if either step fails.
func removeAllAndMkdir(dir string) {
	if err := os.RemoveAll(dir); err != nil {
		log.Fatal(err)
	}
	if err := os.Mkdir(dir, 0755); err != nil {
		log.Fatal(err)
	}
}
