// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The maintnerd command serves project maintainer data from Git,
// Github, and/or Gerrit.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/storage"
	"golang.org/x/build/autocertcache"
	"golang.org/x/build/gerrit"
	"golang.org/x/build/internal/gitauth"
	"golang.org/x/build/maintner"
	"golang.org/x/build/maintner/godata"
	"golang.org/x/build/maintner/maintnerd/apipb"
	"golang.org/x/build/maintner/maintnerd/gcslog"
	"golang.org/x/build/maintner/maintnerd/maintapi"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/http2"
	"golang.org/x/time/rate"
	grpc "grpc.go4.org"
)

var (
	listen          = flag.String("listen", "localhost:6343", "listen address")
	devTLSPort      = flag.Int("dev-tls-port", 0, "if non-zero, port number to run localhost self-signed TLS server")
	autocertDomain  = flag.String("autocert", "", "if non-empty, listen on port 443 and serve a LetsEncrypt TLS cert on this domain")
	autocertBucket  = flag.String("autocert-bucket", "", "if non-empty, Google Cloud Storage bucket to store LetsEncrypt cache in")
	syncQuit        = flag.Bool("sync-and-quit", false, "sync once and quit; don't run a server")
	initQuit        = flag.Bool("init-and-quit", false, "load the mutation log and quit; don't run a server")
	verbose         = flag.Bool("verbose", false, "enable verbose debug output")
	genMut          = flag.Bool("generate-mutations", true, "whether this instance should read from upstream git/gerrit/github and generate new mutations to the end of the log. This requires network access and only one instance can be generating mutation")
	watchGithub     = flag.String("watch-github", "", "Comma-separated list of owner/repo pairs to slurp")
	watchGerrit     = flag.String("watch-gerrit", "", `Comma-separated list of Gerrit projects to watch, each of form "hostname/project" (e.g. "go.googlesource.com/go")`)
	pubsub          = flag.String("pubsub", "", "If non-empty, the golang.org/x/build/cmd/pubsubhelper URL scheme and hostname, without path")
	config          = flag.String("config", "", "If non-empty, the name of a pre-defined config. Valid options are 'go' to be the primary Go server; 'godata' to run the server locally using the godata package, and 'devgo' to act like 'go', but mirror from godata at start-up.")
	dataDir         = flag.String("data-dir", "", "Local directory to write protobuf files to (default $HOME/var/maintnerd)")
	debug           = flag.Bool("debug", false, "Print debug logging information")
	githubRateLimit = flag.Int("github-rate", 10, "Rate to limit GitHub requests (in queries per second, 0 is treated as unlimited)")

	bucket         = flag.String("bucket", "", "if non-empty, Google Cloud Storage bucket to use for log storage")
	migrateGCSFlag = flag.Bool("migrate-disk-to-gcs", false, "[dev] If true, migrate from disk-based logs to GCS logs on start-up, then quit.")
)

func init() {
	flag.Usage = func() {
		os.Stderr.WriteString(`Maintner mirrors, searches, syncs, and serves data from Gerrit, Github, and Git repos.

Maintner gathers data about projects that you want to watch and holds it all in
memory. This way it's easy and fast to search, and you don't have to worry about
retrieving that data from remote APIs.

Maintner is short for "maintainer."

`)
		flag.PrintDefaults()
	}
}

var autocertManager *autocert.Manager

func main() {
	flag.Parse()

	if *autocertDomain != "" {
		if *autocertBucket == "" {
			log.Fatalf("using --autocert requires --autocert-bucket.")
		}
		sc, err := storage.NewClient(context.Background())
		if err != nil {
			log.Fatalf("Creating autocert cache, storage.NewClient: %v", err)
		}
		autocertManager = &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(*autocertDomain),
			Cache:      autocertcache.NewGoogleCloudStorageCache(sc, *autocertBucket),
		}
	}

	if *dataDir == "" {
		*dataDir = filepath.Join(os.Getenv("HOME"), "var", "maintnerd")
		if *bucket == "" {
			if err := os.MkdirAll(*dataDir, 0755); err != nil {
				log.Fatal(err)
			}
			log.Printf("Storing data in implicit directory %s", *dataDir)
		}
	}
	if *migrateGCSFlag && *bucket == "" {
		log.Fatalf("--bucket flag required with --migrate-disk-to-gcs")
	}

	type storage interface {
		maintner.MutationSource
		maintner.MutationLogger
	}
	var logger storage

	corpus := new(maintner.Corpus)
	switch *config {
	case "":
		// Nothing
	case "devgo":
		dir := godata.Dir()
		if err := os.MkdirAll(dir, 0700); err != nil {
			log.Fatal(err)
		}
		log.Printf("Syncing from https://maintner.golang.org/logs to %s", dir)
		mutSrc := maintner.NewNetworkMutationSource("https://maintner.golang.org/logs", dir)
		for evt := range mutSrc.GetMutations(context.Background()) {
			if evt.Err != nil {
				log.Fatal(evt.Err)
			}
			if evt.End {
				break
			}
		}
		syncProdToDevMutationLogs()
		log.Printf("Synced from https://maintner.golang.org/logs.")
		setGoConfig()
	case "go":
		if err := gitauth.Init(); err != nil {
			log.Fatalf("gitauth: %v", err)
		}
		setGoConfig()
	case "godata":
		setGodataConfig()
		var err error
		log.Printf("Using godata corpus...")
		corpus, err = godata.Get(context.Background())
		if err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unknown --config=%s", *config)
	}
	if *genMut {
		if *bucket != "" {
			ctx := context.Background()
			gl, err := gcslog.NewGCSLog(ctx, *bucket)
			if err != nil {
				log.Fatalf("newGCSLog: %v", err)
			}
			gl.RegisterHandlers(http.DefaultServeMux)
			if *migrateGCSFlag {
				diskLog := maintner.NewDiskMutationLogger(*dataDir)
				if err := gl.CopyFrom(diskLog); err != nil {
					log.Fatalf("migrate: %v", err)
				}
				log.Printf("Success.")
				return
			}
			logger = gl
		} else {
			logger = maintner.NewDiskMutationLogger(*dataDir)
		}
		corpus.EnableLeaderMode(logger, *dataDir)
	}
	if *debug {
		corpus.SetDebug()
	}
	corpus.SetVerbose(*verbose)

	if *watchGithub != "" {
		if *githubRateLimit > 0 {
			limit := rate.Every(time.Second / time.Duration(*githubRateLimit))
			corpus.SetGitHubLimiter(rate.NewLimiter(limit, *githubRateLimit))
		}
		for _, pair := range strings.Split(*watchGithub, ",") {
			splits := strings.SplitN(pair, "/", 2)
			if len(splits) != 2 || splits[1] == "" {
				log.Fatalf("Invalid github repo: %s. Should be 'owner/repo,owner2/repo2'", pair)
			}
			token, err := getGithubToken()
			if err != nil {
				log.Fatalf("getting github token: %v", err)
			}
			corpus.TrackGitHub(splits[0], splits[1], token)
		}
	}
	if *watchGerrit != "" {
		for _, project := range strings.Split(*watchGerrit, ",") {
			// token may be empty, that's OK.
			corpus.TrackGerrit(project)
		}
	}

	var ln net.Listener
	var err error
	if !*syncQuit && !*initQuit {
		ln, err = net.Listen("tcp", *listen)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Listening on %v", ln.Addr())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t0 := time.Now()

	if logger != nil {
		if err := corpus.Initialize(ctx, logger); err != nil {
			// TODO: if Initialize only partially syncs the data, we need to delete
			// whatever files it created, since Github returns events newest first
			// and we use the issue updated dates to check whether we need to keep
			// syncing.
			log.Fatal(err)
		}
		initDur := time.Since(t0)

		runtime.GC()
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		log.Printf("Loaded data in %v. Memory: %v MB (%v bytes)", initDur, ms.HeapAlloc>>20, ms.HeapAlloc)
	}
	if *initQuit {
		return
	}

	if *syncQuit {
		if err := corpus.Sync(ctx); err != nil {
			log.Fatalf("corpus.Sync = %v", err)
		}
		if err := corpus.Check(); err != nil {
			log.Fatalf("post-Sync Corpus.Check = %v", err)
		}
		return
	}

	if *pubsub != "" {
		corpus.StartPubSubHelperSubscribe(*pubsub)
	}

	grpcServer := grpc.NewServer()
	apipb.RegisterMaintnerServiceServer(grpcServer, maintapi.NewAPIService(corpus))
	http.Handle("/apipb.MaintnerService/", grpcServer)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
			return
		}
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, `<html>
<body>
<p>
  This is <a href='https://godoc.org/golang.org/x/build/maintner/maintnerd'>maintnerd</a>,
  the <a href='https://godoc.org/golang.org/x/build/maintner'>maintner</a> server.
  See the <a href='https://godoc.org/golang.org/x/build/maintner/godata'>godata package</a> for
  a client.
</p>
<ul>
   <li><a href='/logs'>/logs</a>
</ul>
</body></html>
`)
	})

	errc := make(chan error)
	if *genMut {
		go func() { errc <- fmt.Errorf("Corpus.SyncLoop = %v", corpus.SyncLoop(ctx)) }()
	}
	if ln != nil {
		var handler http.Handler = http.DefaultServeMux
		if autocertManager != nil {
			handler = autocertManager.HTTPHandler(handler)
		}
		go func() { errc <- fmt.Errorf("http.Serve = %v", http.Serve(ln, handler)) }()
	}
	if *autocertDomain != "" {
		go func() { errc <- serveAutocertTLS() }()
	}
	if *devTLSPort != 0 {
		go func() { errc <- serveDevTLS(*devTLSPort) }()
	}

	log.Fatal(<-errc)
}

// Projects to watch when using the "go" config.
var goGitHubProjects = []string{
	"golang/arch",
	"golang/benchmarks",
	"golang/blog",
	"golang/build",
	"golang/crypto",
	"golang/debug",
	"golang/dl",
	"golang/example",
	"golang/exp",
	"golang/gddo",
	"golang/go",
	"golang/image",
	"golang/lint",
	"golang/mobile",
	"golang/net",
	"golang/oauth2",
	"golang/perf",
	"golang/playground",
	"golang/proposal",
	"golang/review",
	"golang/scratch",
	"golang/sublime-build",
	"golang/sublime-config",
	"golang/sync",
	"golang/sys",
	"golang/talks",
	"golang/term",
	"golang/text",
	"golang/time",
	"golang/tools",
	"golang/tour",
	"golang/vgo",
}

func setGoConfig() {
	if *watchGithub != "" {
		log.Fatalf("can't set both --config and --watch-github")
	}
	if *watchGerrit != "" {
		log.Fatalf("can't set both --config and --watch-gerrit")
	}
	*pubsub = "https://pubsubhelper.golang.org"
	*watchGithub = strings.Join(goGitHubProjects, ",")

	gerrc := gerrit.NewClient("https://go-review.googlesource.com/", gerrit.NoAuth)
	projs, err := gerrc.ListProjects(context.Background())
	if err != nil {
		log.Fatalf("error listing Go's gerrit projects: %v", err)
	}
	var buf bytes.Buffer
	buf.WriteString("code.googlesource.com/gocloud,code.googlesource.com/google-api-go-client")
	for _, pi := range projs {
		buf.WriteString(",go.googlesource.com/")
		buf.WriteString(pi.ID)
	}
	*watchGerrit = buf.String()
}

func setGodataConfig() {
	if *watchGithub != "" {
		log.Fatalf("can't set both --config and --watch-github")
	}
	if *watchGerrit != "" {
		log.Fatalf("can't set both --config and --watch-gerrit")
	}
	*genMut = false
}

func getGithubToken() (string, error) {
	if metadata.OnGCE() {
		token, err := metadata.ProjectAttributeValue("maintner-github-token")
		if err == nil {
			return token, nil
		}
		log.Printf("getting GCE metadata 'maintner-github-token': %v", err)
		log.Printf("falling back to github token from file.")
	}

	tokenFile := filepath.Join(os.Getenv("HOME"), ".github-issue-token")
	slurp, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		return "", err
	}
	f := strings.SplitN(strings.TrimSpace(string(slurp)), ":", 2)
	if len(f) != 2 || f[0] == "" || f[1] == "" {
		return "", fmt.Errorf("Expected token file %s to be of form <username>:<token>", tokenFile)
	}
	token := f[1]
	return token, nil
}

func serveDevTLS(port int) error {
	ln, err := net.Listen("tcp", "localhost:"+strconv.Itoa(port))
	if err != nil {
		return err
	}
	defer ln.Close()
	log.Printf("Serving self-signed TLS at https://%s", ln.Addr())
	// Abuse httptest for its localhost TLS setup code:
	ts := httptest.NewUnstartedServer(http.DefaultServeMux)
	// Ditch the provided listener, replace with our own:
	ts.Listener.Close()
	ts.Listener = ln
	ts.TLS = &tls.Config{
		NextProtos:         []string{"h2", "http/1.1"},
		InsecureSkipVerify: true,
	}
	ts.StartTLS()

	select {}
}

func serveAutocertTLS() error {
	if *autocertBucket == "" {
		return fmt.Errorf("using --autocert requires --autocert-bucket.")
	}
	ln, err := net.Listen("tcp", ":443")
	if err != nil {
		return err
	}
	defer ln.Close()
	config := &tls.Config{
		GetCertificate: autocertManager.GetCertificate,
		NextProtos:     []string{"h2", "http/1.1"},
	}
	tlsLn := tls.NewListener(tcpKeepAliveListener{ln.(*net.TCPListener)}, config)
	server := &http.Server{
		Addr: ln.Addr().String(),
	}
	if err := http2.ConfigureServer(server, nil); err != nil {
		log.Fatalf("http2.ConfigureServer: %v", err)
	}
	return server.Serve(tlsLn)
}

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

func syncProdToDevMutationLogs() {
	src := godata.Dir()
	dst := *dataDir

	want := map[string]int64{} // basename => size

	srcFis, err := ioutil.ReadDir(src)
	if err != nil {
		log.Fatal(err)
	}
	dstFis, err := ioutil.ReadDir(dst)
	if err != nil {
		log.Fatal(err)
	}

	for _, fi := range srcFis {
		name := fi.Name()
		if !strings.HasSuffix(name, ".mutlog") {
			continue
		}
		// The DiskMutationLogger (as we'l use in the dst dir)
		// prepends "maintner-".  So prepend that here ahead
		// of time, even though the network mutation source's
		// cache doesn't.
		want["maintner-"+name] = fi.Size()
	}

	for _, fi := range dstFis {
		name := fi.Name()
		if !strings.HasSuffix(name, ".mutlog") {
			continue
		}
		if want[name] == fi.Size() {
			delete(want, name)
			continue
		}
		log.Printf("dst file %q unwanted", name)
		if err := os.Remove(filepath.Join(dst, name)); err != nil {
			log.Fatal(err)
		}
	}

	for name := range want {
		log.Printf("syncing %s from %s to %s", name, src, dst)
		slurp, err := ioutil.ReadFile(filepath.Join(src, strings.TrimPrefix(name, "maintner-")))
		if err != nil {
			log.Fatal(err)
		}
		if err := ioutil.WriteFile(filepath.Join(dst, name), slurp, 0644); err != nil {
			log.Fatal(err)
		}
	}
}
