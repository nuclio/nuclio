// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/coreos/go-systemd/activation"
	"github.com/coreos/go-systemd/daemon"
	"golang.org/x/build/autocertcache"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

var (
	dir     = flag.String("d", "/tmp/vcweb", "directory holding vcweb data")
	staging = flag.Bool("staging", false, "use staging letsencrypt server")
)

var buildInfo string

func usage() {
	fmt.Fprintf(os.Stderr, "usage: vcsweb [-d dir] [-staging]\n")
	os.Exit(2)
}

var isLoadDir = map[string]bool{
	"go":     true,
	"git":    true,
	"hg":     true,
	"svn":    true,
	"fossil": true,
	"bzr":    true,
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 0 {
		usage()
	}

	if err := os.MkdirAll(*dir, 0777); err != nil {
		log.Fatal(err)
	}

	http.Handle("/go/", http.StripPrefix("/go/", http.FileServer(http.Dir(filepath.Join(*dir, "go")))))
	http.Handle("/git/", gitHandler())
	http.Handle("/hg/", hgHandler())
	http.Handle("/svn/", svnHandler())
	http.Handle("/fossil/", fossilHandler())
	http.Handle("/bzr/", bzrHandler())

	handler := logger(http.HandlerFunc(loadAndHandle))

	// If running under systemd, listen on 80 and 443 and serve TLS.
	if listeners, _ := activation.ListenersWithNames(); len(listeners) == 2 {
		httpListener := listeners["vcweb-http.socket"][0]
		httpsListener := listeners["vcweb-https.socket"][0]

		go func() {
			log.Fatal(http.Serve(httpListener, handler))
		}()
		dir := acme.LetsEncryptURL
		if *staging {
			dir = "https://acme-staging.api.letsencrypt.org/directory"
		}
		m := autocert.Manager{
			Client:     &acme.Client{DirectoryURL: dir},
			Cache:      autocertcache.NewGoogleCloudStorageCache(client, "vcs-test-autocert"),
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist("vcs-test.golang.org"),
		}
		s := &http.Server{
			Addr:    ":https",
			Handler: handler,
			TLSConfig: &tls.Config{
				MinVersion:     tls.VersionSSL30,
				GetCertificate: fallbackSNI(m.GetCertificate, "vcs-test.golang.org"),
				NextProtos: []string{
					"h2", "http/1.1", // enable HTTP/2
					acme.ALPNProto, // enable tls-alpn ACME challenges
				},
			},
		}

		dt, err := daemon.SdWatchdogEnabled(true)
		if err != nil {
			log.Fatal(err)
		}

		daemon.SdNotify(false, "READY=1")
		go func() {
			for range time.NewTicker(dt / 2).C {
				daemon.SdNotify(false, "WATCHDOG=1")
			}
		}()
		log.Fatal(s.ServeTLS(httpsListener, "", ""))
	}

	// Local development on :8088.
	l, err := net.Listen("tcp", "127.0.0.1:8088")
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(http.Serve(l, handler))
}

var nameRE = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)

func loadAndHandle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/tls" {
		handleTLS(w, r)
		return
	}
	addTLSLog(w, r)
	if r.URL.Path == "/" {
		overview(w, r)
		return
	}
	elem := strings.Split(r.URL.Path, "/")
	if len(elem) >= 3 && elem[0] == "" && isLoadDir[elem[1]] && nameRE.MatchString(elem[2]) {
		loadFS(elem[1], elem[2], r.URL.Query().Get("vcweb-force-reload") == "1" || r.URL.Query().Get("go-get") == "1")
	}
	http.DefaultServeMux.ServeHTTP(w, r)
}

func overview(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<html>\n")
	fmt.Fprintf(w, "<title>vcs-test.golang.org</title>\n<pre>\n")
	fmt.Fprintf(w, "<b>vcs-test.golang.org</b>\n\n")
	fmt.Fprintf(w, "This server serves various version control repos for testing the go command.\n\n")

	fmt.Fprintf(w, "Date: %s\n", time.Now().Format(time.UnixDate))
	fmt.Fprintf(w, "Build: %s\n\n", html.EscapeString(buildInfo))

	fmt.Fprintf(w, "<b>cache</b>\n")

	var all []string
	cache.Lock()
	for name, entry := range cache.entry {
		all = append(all, fmt.Sprintf("%s\t%x\t%s\n", name, entry.md5, entry.expire.Format(time.UnixDate)))
	}
	cache.Unlock()
	sort.Strings(all)
	tw := tabwriter.NewWriter(w, 1, 8, 1, '\t', 0)
	for _, line := range all {
		tw.Write([]byte(line))
	}
	tw.Flush()
}

func fallbackSNI(getCert func(*tls.ClientHelloInfo) (*tls.Certificate, error), host string) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		saveHello(hello)
		if hello.ServerName == "" {
			h := *hello
			hello = &h
			hello.ServerName = host
		}
		return getCert(hello)
	}
}

type loggingResponseWriter struct {
	code int
	size int64
	http.ResponseWriter
}

func (l *loggingResponseWriter) WriteHeader(code int) {
	l.code = code
	l.ResponseWriter.WriteHeader(code)
}

func (l *loggingResponseWriter) Write(data []byte) (int, error) {
	n, err := l.ResponseWriter.Write(data)
	l.size += int64(n)
	return n, err
}

func dashOr(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func logger(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := &loggingResponseWriter{
			code:           200,
			ResponseWriter: w,
		}
		startTime := time.Now().Format("02/Jan/2006:15:04:05 -0700")
		defer func() {
			err := recover()
			if err != nil {
				l.code = 999
			}
			fmt.Fprintf(os.Stderr, "%s - - [%s] %q %03d %d %q %q %q\n",
				dashOr(r.RemoteAddr),
				startTime,
				r.Method+" "+r.URL.String()+" "+r.Proto,
				l.code,
				l.size,
				r.Header.Get("Referer"),
				r.Header.Get("User-Agent"),
				r.Host)
			if err != nil {
				panic(err)
			}
		}()
		h.ServeHTTP(l, r)
	}
}
