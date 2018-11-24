// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code related to remote buildlets. See x/build/remote-buildlet.txt

package main // import "golang.org/x/build/cmd/coordinator"

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"cloud.google.com/go/compute/metadata"

	"github.com/gliderlabs/ssh"
	"github.com/kr/pty"
	"golang.org/x/build/buildlet"
	"golang.org/x/build/dashboard"
	"golang.org/x/build/internal/gophers"
	gossh "golang.org/x/crypto/ssh"
)

var (
	remoteBuildlets = struct {
		sync.Mutex
		m map[string]*remoteBuildlet // keyed by buildletName
	}{m: map[string]*remoteBuildlet{}}

	cleanTimer *time.Timer
)

const (
	remoteBuildletIdleTimeout   = 30 * time.Minute
	remoteBuildletCleanInterval = time.Minute
)

func init() {
	cleanTimer = time.AfterFunc(remoteBuildletCleanInterval, expireBuildlets)
}

type remoteBuildlet struct {
	User        string // "user-foo" build key
	Name        string // dup of key
	HostType    string
	BuilderType string // default builder config to use if not overwritten
	Created     time.Time
	Expires     time.Time

	buildlet *buildlet.Client
}

// renew renews rb's idle timeout if ctx hasn't expired.
// renew should run in its own goroutine.
func (rb *remoteBuildlet) renew(ctx context.Context) {
	remoteBuildlets.Lock()
	defer remoteBuildlets.Unlock()
	select {
	case <-ctx.Done():
		return
	default:
	}
	if got := remoteBuildlets.m[rb.Name]; got == rb {
		rb.Expires = time.Now().Add(remoteBuildletIdleTimeout)
		time.AfterFunc(time.Minute, func() { rb.renew(ctx) })
	}
}

func addRemoteBuildlet(rb *remoteBuildlet) (name string) {
	remoteBuildlets.Lock()
	defer remoteBuildlets.Unlock()
	n := 0
	for {
		name = fmt.Sprintf("%s-%s-%d", rb.User, rb.BuilderType, n)
		if _, ok := remoteBuildlets.m[name]; ok {
			n++
		} else {
			remoteBuildlets.m[name] = rb
			return name
		}
	}
}

func expireBuildlets() {
	defer cleanTimer.Reset(remoteBuildletCleanInterval)
	remoteBuildlets.Lock()
	defer remoteBuildlets.Unlock()
	now := time.Now()
	for name, rb := range remoteBuildlets.m {
		if !rb.Expires.IsZero() && rb.Expires.Before(now) {
			go rb.buildlet.Close()
			delete(remoteBuildlets.m, name)
		}
	}
}

// always wrapped in requireBuildletProxyAuth.
func handleBuildletCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", 400)
		return
	}
	const serverVersion = "20160922" // sent by cmd/gomote via buildlet/remote.go
	if version := r.FormValue("version"); version < serverVersion {
		http.Error(w, fmt.Sprintf("gomote client version %q is too old; predates server version %q", version, serverVersion), 400)
		return
	}
	builderType := r.FormValue("builderType")
	if builderType == "" {
		http.Error(w, "missing 'builderType' parameter", 400)
		return
	}
	bconf, ok := dashboard.Builders[builderType]
	if !ok {
		http.Error(w, "unknown builder type in 'builderType' parameter", 400)
		return
	}
	user, _, _ := r.BasicAuth()
	pool := poolForConf(bconf)

	var closeNotify <-chan bool
	if cn, ok := w.(http.CloseNotifier); ok {
		closeNotify = cn.CloseNotify()
	}

	ctx := context.WithValue(context.Background(), buildletTimeoutOpt{}, time.Duration(0))
	ctx, cancel := context.WithCancel(ctx)
	// NOTE: don't defer close this cancel. If the context is
	// closed, the pod is destroyed.
	// TODO: clean this up.

	// Doing a release?
	if user == "release" || user == "adg" || user == "bradfitz" {
		ctx = context.WithValue(ctx, highPriorityOpt{}, true)
	}

	resc := make(chan *buildlet.Client)
	errc := make(chan error)
	go func() {
		bc, err := pool.GetBuildlet(ctx, bconf.HostType, loggerFunc(func(event string, optText ...string) {
			var extra string
			if len(optText) > 0 {
				extra = " " + optText[0]
			}
			log.Printf("creating buildlet %s for %s: %s%s", bconf.HostType, user, event, extra)
		}))
		if bc != nil {
			resc <- bc
			return
		}
		errc <- err
	}()
	for {
		select {
		case bc := <-resc:
			rb := &remoteBuildlet{
				User:        user,
				BuilderType: builderType,
				HostType:    bconf.HostType,
				buildlet:    bc,
				Created:     time.Now(),
				Expires:     time.Now().Add(remoteBuildletIdleTimeout),
			}
			rb.Name = addRemoteBuildlet(rb)
			jenc, err := json.MarshalIndent(rb, "", "  ")
			if err != nil {
				http.Error(w, err.Error(), 500)
				log.Print(err)
				return
			}
			log.Printf("created buildlet %v for %v (%s)", rb.Name, rb.User, bc.String())
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			jenc = append(jenc, '\n')
			w.Write(jenc)
			return
		case err := <-errc:
			log.Printf("error creating buildlet: %v", err)
			http.Error(w, err.Error(), 500)
			return
		case <-closeNotify:
			log.Printf("client went away during buildlet create request")
			cancel()
			closeNotify = nil // unnecessary, but habit.
		}
	}
}

// always wrapped in requireBuildletProxyAuth.
func handleBuildletList(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "GET required", 400)
		return
	}
	res := make([]*remoteBuildlet, 0) // so it's never JSON "null"
	remoteBuildlets.Lock()
	defer remoteBuildlets.Unlock()
	user, _, _ := r.BasicAuth()
	for _, rb := range remoteBuildlets.m {
		if rb.User == user {
			res = append(res, rb)
		}
	}
	sort.Sort(byBuildletName(res))
	jenc, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	jenc = append(jenc, '\n')
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(jenc)
}

type byBuildletName []*remoteBuildlet

func (s byBuildletName) Len() int           { return len(s) }
func (s byBuildletName) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s byBuildletName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func remoteBuildletStatus() string {
	remoteBuildlets.Lock()
	defer remoteBuildlets.Unlock()

	if len(remoteBuildlets.m) == 0 {
		return "<i>(none)</i>"
	}

	var buf bytes.Buffer
	var all []*remoteBuildlet
	for _, rb := range remoteBuildlets.m {
		all = append(all, rb)
	}
	sort.Sort(byBuildletName(all))

	buf.WriteString("<ul>")
	for _, rb := range all {
		fmt.Fprintf(&buf, "<li><b>%s</b>, created %v ago, expires in %v</li>\n",
			html.EscapeString(rb.Name),
			time.Since(rb.Created), rb.Expires.Sub(time.Now()))
	}
	buf.WriteString("</ul>")

	return buf.String()
}

// httpRouter separates out HTTP traffic being proxied
// to buildlets on behalf of remote clients from traffic
// destined for the coordinator itself (the default).
type httpRouter struct{}

func (httpRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Buildlet-Proxy") != "" {
		requireBuildletProxyAuth(http.HandlerFunc(proxyBuildletHTTP)).ServeHTTP(w, r)
	} else {
		http.DefaultServeMux.ServeHTTP(w, r)
	}
}

func proxyBuildletHTTP(w http.ResponseWriter, r *http.Request) {
	if r.TLS == nil {
		http.Error(w, "https required", http.StatusBadRequest)
		return
	}
	buildletName := r.Header.Get("X-Buildlet-Proxy")
	if buildletName == "" {
		http.Error(w, "missing X-Buildlet-Proxy; server misconfig", http.StatusInternalServerError)
		return
	}
	remoteBuildlets.Lock()
	rb, ok := remoteBuildlets.m[buildletName]
	if ok {
		rb.Expires = time.Now().Add(remoteBuildletIdleTimeout)
	}
	remoteBuildlets.Unlock()
	if !ok {
		http.Error(w, "unknown or expired buildlet", http.StatusBadGateway)
		return
	}
	user, _, _ := r.BasicAuth()
	if rb.User != user {
		http.Error(w, "you don't own that buildlet", http.StatusUnauthorized)
		return
	}

	if r.Method == "POST" && r.URL.Path == "/halt" {
		err := rb.buildlet.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		rb.buildlet.Close()
		remoteBuildlets.Lock()
		delete(remoteBuildlets.m, buildletName)
		remoteBuildlets.Unlock()
		return
	}

	outReq, err := http.NewRequest(r.Method, rb.buildlet.URL()+r.URL.Path+"?"+r.URL.RawQuery, r.Body)
	if err != nil {
		log.Printf("bad proxy request: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	outReq.Header = r.Header
	outReq.ContentLength = r.ContentLength
	proxy := &httputil.ReverseProxy{
		Director:      func(*http.Request) {}, // nothing
		Transport:     rb.buildlet.ProxyRoundTripper(),
		FlushInterval: 500 * time.Millisecond,
	}
	proxy.ServeHTTP(w, outReq)
}

func requireBuildletProxyAuth(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			http.Error(w, "missing required authentication", 400)
			return
		}
		if !strings.HasPrefix(user, "user-") || builderKey(user) != pass {
			if *mode == "dev" {
				log.Printf("ignoring gomote authentication failure for %q in dev mode", user)
			} else {
				http.Error(w, "bad username or password", 401)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

var sshPrivateKeyFile string

func writeSSHPrivateKeyToTempFile(key []byte) (path string, err error) {
	tf, err := ioutil.TempFile("", "ssh-priv-key")
	if err != nil {
		return "", err
	}
	if err := tf.Chmod(0600); err != nil {
		return "", err
	}
	if _, err := tf.Write(key); err != nil {
		return "", err
	}
	return tf.Name(), tf.Close()
}

func listenAndServeSSH() {
	const listenAddr = ":2222" // TODO: flag if ever necessary?
	var hostKey []byte
	var err error
	if *mode == "dev" {
		sshPrivateKeyFile = filepath.Join(os.Getenv("HOME"), "keys", "id_gomotessh_rsa")
		hostKey, err = ioutil.ReadFile(sshPrivateKeyFile)
		if os.IsNotExist(err) {
			log.Printf("SSH host key file %s doesn't exist; not running SSH server.", sshPrivateKeyFile)
			return
		}
		if err != nil {
			log.Fatal(err)
		}
	} else {
		if storageClient == nil {
			log.Printf("GCS storage client not available; not running SSH server.")
			return
		}
		r, err := storageClient.Bucket(buildEnv.BuildletBucket).Object("coordinator-gomote-ssh.key").NewReader(context.Background())
		if err != nil {
			log.Printf("Failed to read ssh host key: %v; not running SSH server.", err)
			return
		}
		hostKey, err = ioutil.ReadAll(r)
		if err != nil {
			log.Printf("Failed to read ssh host key: %v; not running SSH server.", err)
			return
		}
		sshPrivateKeyFile, err = writeSSHPrivateKeyToTempFile(hostKey)
		log.Printf("ssh: writeSSHPrivateKeyToTempFile = %v, %v", sshPrivateKeyFile, err)
		if err != nil {
			log.Printf("error writing ssh private key to temp file: %v; not running SSH server", err)
			return
		}
	}
	signer, err := gossh.ParsePrivateKey(hostKey)
	if err != nil {
		log.Printf("failed to parse SSH host key: %v; running running SSH server", err)
		return
	}

	s := &ssh.Server{
		Addr:             listenAddr,
		Handler:          handleIncomingSSHPostAuth,
		PublicKeyHandler: handleSSHPublicKeyAuth,
	}
	s.AddHostKey(signer)

	log.Printf("running SSH server on %s", listenAddr)
	err = s.ListenAndServe()
	log.Printf("SSH server ended with error: %v", err)
	// TODO: make ListenAndServe errors Fatal, once it has a proven track record. starting paranoid.
}

func handleSSHPublicKeyAuth(ctx ssh.Context, key ssh.PublicKey) bool {
	inst := ctx.User() // expected to be of form "user-USER-goos-goarch-etc"
	user := userFromGomoteInstanceName(inst)
	if user == "" {
		return false
	}
	// Map the gomote username to the github username, and use the
	// github user's public ssh keys for authentication. This is
	// mostly of laziness and pragmatism, not wanting to invent or
	// maintain a new auth mechanism or password/key registry.
	githubUser := gophers.GithubOfGomoteUser(user)
	keys := githubPublicKeys(githubUser)
	for _, authKey := range keys {
		if ssh.KeysEqual(key, authKey.PublicKey) {
			log.Printf("for instance %q, github user %q key matched: %s", inst, githubUser, authKey.AuthorizedLine)
			return true
		}
	}
	return false
}

func handleIncomingSSHPostAuth(s ssh.Session) {
	inst := s.User()
	user := userFromGomoteInstanceName(inst)

	requestedMutable := strings.HasPrefix(inst, "mutable-")
	if requestedMutable {
		inst = strings.TrimPrefix(inst, "mutable-")
	}

	ptyReq, winCh, isPty := s.Pty()
	if !isPty {
		fmt.Fprintf(s, "scp etc not yet supported; https://golang.org/issue/21140\n")
		return
	}

	pubKey, err := metadata.ProjectAttributeValue("gomote-ssh-public-key")
	if err != nil || pubKey == "" {
		if err == nil {
			err = errors.New("not found")
		}
		fmt.Fprintf(s, "failed to get GCE gomote-ssh-public-key: %v\n", err)
		return
	}

	remoteBuildlets.Lock()
	rb, ok := remoteBuildlets.m[inst]
	remoteBuildlets.Unlock()
	if !ok {
		fmt.Fprintf(s, "unknown instance %q", inst)
		return
	}

	hostType := rb.HostType
	hostConf, ok := dashboard.Hosts[hostType]
	if !ok {
		fmt.Fprintf(s, "instance %q has unknown host type %q\n", inst, hostType)
		return
	}

	bconf, ok := dashboard.Builders[rb.BuilderType]
	if !ok {
		fmt.Fprintf(s, "instance %q has unknown builder type %q\n", inst, rb.BuilderType)
		return
	}

	ctx, cancel := context.WithCancel(s.Context())
	defer cancel()
	go rb.renew(ctx)

	sshUser := hostConf.SSHUsername
	useLocalSSHProxy := bconf.GOOS() != "plan9"
	if sshUser == "" && useLocalSSHProxy {
		fmt.Fprintf(s, "instance %q host type %q does not have SSH configured\n", inst, hostType)
		return
	}
	if !hostConf.IsHermetic() && !requestedMutable {
		fmt.Fprintf(s, "WARNING: instance %q host type %q is not currently\n", inst, hostType)
		fmt.Fprintf(s, "configured to have a hermetic filesystem per boot.\n")
		fmt.Fprintf(s, "You must be careful not to modify machine state\n")
		fmt.Fprintf(s, "that will affect future builds. Do you agree? If so,\n")
		fmt.Fprintf(s, "run gomote ssh --i-will-not-break-the-host <INST>\n")
		return
	}

	log.Printf("connecting to ssh to instance %q ...", inst)

	fmt.Fprintf(s, "# Welcome to the gomote ssh proxy, %s.\n", user)
	fmt.Fprintf(s, "# Connecting to/starting remote ssh...\n")
	fmt.Fprintf(s, "#\n")

	var localProxyPort int
	if useLocalSSHProxy {
		sshConn, err := rb.buildlet.ConnectSSH(sshUser, pubKey)
		log.Printf("buildlet(%q).ConnectSSH = %T, %v", inst, sshConn, err)
		if err != nil {
			fmt.Fprintf(s, "failed to connect to ssh on %s: %v\n", inst, err)
			return
		}
		defer sshConn.Close()

		// Now listen on some localhost port that we'll proxy to sshConn.
		// The openssh ssh command line tool will connect to this IP.
		ln, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			fmt.Fprintf(s, "local listen error: %v\n", err)
			return
		}
		localProxyPort = ln.Addr().(*net.TCPAddr).Port
		log.Printf("ssh local proxy port for %s: %v", inst, localProxyPort)
		var lnCloseOnce sync.Once
		lnClose := func() { lnCloseOnce.Do(func() { ln.Close() }) }
		defer lnClose()

		// Accept at most one connection from localProxyPort and proxy
		// it to sshConn.
		go func() {
			c, err := ln.Accept()
			lnClose()
			if err != nil {
				return
			}
			defer c.Close()
			errc := make(chan error, 1)
			go func() {
				_, err := io.Copy(c, sshConn)
				errc <- err
			}()
			go func() {
				_, err := io.Copy(sshConn, c)
				errc <- err
			}()
			err = <-errc
		}()
	}
	workDir, err := rb.buildlet.WorkDir()
	if err != nil {
		fmt.Fprintf(s, "Error getting WorkDir: %v\n", err)
		return
	}
	ip, _, ipErr := net.SplitHostPort(rb.buildlet.IPPort())

	fmt.Fprintf(s, "# `gomote push` and the builders use:\n")
	fmt.Fprintf(s, "# - workdir: %s\n", workDir)
	fmt.Fprintf(s, "# - GOROOT: %s/go\n", workDir)
	fmt.Fprintf(s, "# - GOPATH: %s/gopath\n", workDir)
	fmt.Fprintf(s, "# - env: %s\n", strings.Join(bconf.Env(), " ")) // TODO: shell quote?
	fmt.Fprintf(s, "# Happy debugging.\n")

	log.Printf("ssh to %s: starting ssh -p %d for %s@localhost", inst, localProxyPort, sshUser)
	var cmd *exec.Cmd
	switch bconf.GOOS() {
	default:
		cmd = exec.Command("ssh",
			"-p", strconv.Itoa(localProxyPort),
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "StrictHostKeyChecking=no",
			"-i", sshPrivateKeyFile,
			sshUser+"@localhost")
	case "plan9":
		fmt.Fprintf(s, "# Plan9 user/pass: glenda/glenda123\n")
		if ipErr != nil {
			fmt.Fprintf(s, "# Failed to get IP out of %q: %v\n", rb.buildlet.IPPort(), err)
			return
		}
		cmd = exec.Command("/usr/local/bin/drawterm",
			"-a", ip, "-c", ip, "-u", "glenda", "-k", "user=glenda")
	}
	cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))
	f, err := pty.Start(cmd)
	if err != nil {
		log.Printf("running ssh client to %s: %v", inst, err)
		return
	}
	defer f.Close()
	go func() {
		for win := range winCh {
			setWinsize(f, win.Width, win.Height)
		}
	}()
	go func() {
		io.Copy(f, s) // stdin
	}()
	io.Copy(s, f) // stdout
	cmd.Process.Kill()
	cmd.Wait()
}

func setWinsize(f *os.File, w, h int) {
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})))
}

// userFromGomoteInstanceName returns the username part of a gomote
// remote instance name.
//
// The instance name is of two forms. The normal form is:
//
//     user-bradfitz-linux-amd64-0
//
// The overloaded form to convey that the user accepts responsibility
// for changes to the underlying host is to prefix the same instance
// name with the string "mutable-", such as:
//
//     mutable-user-bradfitz-darwin-amd64-10_8-0
//
// The mutable part is ignored by this function.
func userFromGomoteInstanceName(name string) string {
	name = strings.TrimPrefix(name, "mutable-")
	if !strings.HasPrefix(name, "user-") {
		return ""
	}
	user := name[len("user-"):]
	hyphen := strings.IndexByte(user, '-')
	if hyphen == -1 {
		return ""
	}
	return user[:hyphen]
}

// authorizedKey is a Github user's SSH authorized key, in both string and parsed format.
type authorizedKey struct {
	AuthorizedLine string // e.g. "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILj8HGIG9NsT34PHxO8IBq0riSBv7snp30JM8AanBGoV"
	PublicKey      ssh.PublicKey
}

func githubPublicKeys(user string) []authorizedKey {
	// TODO: caching, rate limiting.
	req, err := http.NewRequest("GET", "https://github.com/"+user+".keys", nil)
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("getting %s github keys: %v", user, err)
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil
	}
	var keys []authorizedKey
	bs := bufio.NewScanner(res.Body)
	for bs.Scan() {
		key, _, _, _, err := ssh.ParseAuthorizedKey(bs.Bytes())
		if err != nil {
			log.Printf("parsing github user %q key %q: %v", user, bs.Text(), err)
			continue
		}
		keys = append(keys, authorizedKey{
			PublicKey:      key,
			AuthorizedLine: strings.TrimSpace(bs.Text()),
		})
	}
	if err := bs.Err(); err != nil {
		return nil
	}
	return keys
}
