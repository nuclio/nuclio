// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build BROKEN

// I get:
// $ go test -v
// ./reverse_test.go:15: can't find import: "golang.org/x/build/cmd/buildlet"

package main

import (
	"flag"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	buildletmain "golang.org/x/build/cmd/buildlet"
)

func TestReverseDial(t *testing.T) {
	*mode = "dev"
	http.HandleFunc("/reverse", handleReverse)

	ln, err := net.Listen("tcp", "")
	if err != nil {
		t.Fatalf(`net.Listen(":"): %v`, err)
	}
	t.Logf("listening on %s...", ln.Addr())
	go serveTLS(ln)

	wantModes := "goos-goarch-test1,goos-goarch-test2"
	flag.CommandLine.Set("coordinator", ln.Addr().String())
	flag.CommandLine.Set("reverse", wantModes)

	ch := make(chan []string)
	registerBuildlet = func(modes []string) { ch <- modes }
	go buildletmain.TestDialCoordinator()

	select {
	case modes := <-ch:
		gotModes := strings.Join(modes, ",")
		if gotModes != wantModes {
			t.Errorf("want buildlet registered with modes %q, got %q", wantModes, gotModes)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for buildlet registration")
	}
}
