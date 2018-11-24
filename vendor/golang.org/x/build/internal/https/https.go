// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package https contains helpers for starting an HTTPS server.
package https // import "golang.org/x/build/internal/https"

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/build/autocertcache"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/http2"
)

// Options are the configuration parameters for the HTTP(S) server.
type Options struct {
	// Addr specifies the host and port the server should listen on.
	Addr string

	// AutocertCacheBucket specifies the name of the GCS bucket for
	// Let’s Encrypt to use. If this is not specified, then HTTP traffic is
	// served on Addr.
	AutocertCacheBucket string
}

var defaultOptions = &Options{
	Addr: "localhost:6343",
}

// ListenAndServe serves the given handler by HTTPS (and HTTP, redirecting to
// HTTPS) using the provided options.
func ListenAndServe(handler http.Handler, opt *Options) error {
	if opt == nil {
		opt = defaultOptions
	}
	ln, err := net.Listen("tcp", opt.Addr)
	if err != nil {
		return fmt.Errorf(`net.Listen("tcp", %q): %v`, opt.Addr, err)
	}

	errc := make(chan error)
	if ln != nil {
		go func() {
			if opt.AutocertCacheBucket != "" {
				handler = http.HandlerFunc(redirectToHTTPS)
			}
			errc <- fmt.Errorf("http.Serve = %v", http.Serve(ln, handler))
		}()
	}
	if opt.AutocertCacheBucket != "" {
		go func() { errc <- serveAutocertTLS(handler, opt.AutocertCacheBucket) }()
	}
	return <-errc
}

// redirectToHTTPS will redirect to the https version of the URL requested. If
// r.TLS is set or r.Host is empty, a 404 not found response is sent.
func redirectToHTTPS(w http.ResponseWriter, r *http.Request) {
	if r.TLS != nil || r.Host == "" {
		http.NotFound(w, r)
		return
	}

	http.Redirect(w, r, "https://"+r.Host+r.RequestURI, http.StatusFound)
}

// serveAutocertTLS serves the handler h on port 443 using the given GCS bucket
// for its autocert cache. It will only serve on domains of the form *.golang.org.
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
	const hostSuffix = ".golang.org"
	m := autocert.Manager{
		Prompt: autocert.AcceptTOS,
		HostPolicy: func(ctx context.Context, host string) error {
			if !strings.HasSuffix(host, hostSuffix) {
				return fmt.Errorf("refusing to serve autocert on provided domain (%q), must have the suffix %q",
					host, hostSuffix)
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
		return fmt.Errorf("http2.ConfigureServer: %v", err)
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
