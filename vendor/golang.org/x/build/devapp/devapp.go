// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Devapp is the server running dev.golang.org. It shows open bugs and code
// reviews and other useful dashboards for Go developers.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/build/autocertcache"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/http2"
)

func init() {
	flag.Usage = func() {
		os.Stderr.WriteString("devapp generates the dashboard that powers dev.golang.org.\n")
		flag.PrintDefaults()
	}
}

func main() {
	var (
		listen         = flag.String("listen", "localhost:6343", "listen address")
		devTLSPort     = flag.Int("dev-tls-port", 0, "if non-zero, port number to run localhost self-signed TLS server")
		autocertBucket = flag.String("autocert-bucket", "", "if non-empty, listen on port 443 and serve a LetsEncrypt TLS cert using this Google Cloud Storage bucket as a cache")
		staticDir      = flag.String("static-dir", "./static/", "location of static directory relative to binary location")
		templateDir    = flag.String("template-dir", "./templates/", "location of templates directory relative to binary location")
	)
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	s := newServer(http.NewServeMux(), *staticDir, *templateDir)
	ctx := context.Background()
	if err := s.initCorpus(ctx); err != nil {
		log.Fatalf("Could not init corpus: %v", err)
	}
	go s.corpusUpdateLoop(ctx)

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatalf("Error listening on %s: %v", *listen, err)
	}
	log.Printf("Listening on %s", ln.Addr())

	errc := make(chan error)
	if ln != nil {
		go func() {
			handler := http.Handler(s)
			if *autocertBucket != "" {
				handler = http.HandlerFunc(redirectHTTP)
			}
			errc <- fmt.Errorf("http.Serve = %v", http.Serve(ln, handler))
		}()
	}
	if *autocertBucket != "" {
		go func() { errc <- serveAutocertTLS(s, *autocertBucket) }()
	}
	if *devTLSPort != 0 {
		go func() { errc <- serveDevTLS(s, *devTLSPort) }()
	}

	log.Fatal(<-errc)
}

func redirectHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		handleHealthz(w, r)
		return
	}
	if r.TLS != nil || r.Host == "" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "https://"+r.Host+r.RequestURI, http.StatusFound)
}

func serveDevTLS(h http.Handler, port int) error {
	ln, err := net.Listen("tcp", "localhost:"+strconv.Itoa(port))
	if err != nil {
		return err
	}
	defer ln.Close()
	log.Printf("Serving self-signed TLS at https://%s", ln.Addr())
	// Abuse httptest for its localhost TLS setup code:
	ts := httptest.NewUnstartedServer(h)
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

func serveAutocertTLS(h http.Handler, bucket string) error {
	ln, err := net.Listen("tcp", ":443")
	if err != nil {
		return err
	}
	defer ln.Close()
	sc, err := storage.NewClient(context.Background())
	if err != nil {
		return fmt.Errorf("storage.NewClient: %v", err)
	}
	m := autocert.Manager{
		Prompt: autocert.AcceptTOS,
		HostPolicy: func(ctx context.Context, host string) error {
			if !strings.HasSuffix(host, ".golang.org") {
				return errors.New("refusing to serve autocert on provided domain")
			}
			return nil
		},
		Cache: autocertcache.NewGoogleCloudStorageCache(sc, bucket),
	}
	config := &tls.Config{
		GetCertificate: m.GetCertificate,
		NextProtos:     []string{"h2", "http/1.1"},
	}
	tlsLn := tls.NewListener(tcpKeepAliveListener{ln.(*net.TCPListener)}, config)
	server := &http.Server{
		Addr:    ln.Addr().String(),
		Handler: h,
	}
	if err := http2.ConfigureServer(server, nil); err != nil {
		log.Fatalf("http2.ConfigureServer: %v", err)
	}
	log.Printf("Serving TLS at %s", tlsLn.Addr())
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
