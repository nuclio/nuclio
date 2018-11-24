// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

var testServer = newServer(http.DefaultServeMux, "./static/", "./templates/")

func TestStaticAssetsFound(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	testServer.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected code %d, got %d", http.StatusOK, w.Code)
	}
	if hdr := w.Header().Get("Content-Type"); hdr != "text/html; charset=utf-8" {
		t.Errorf("incorrect Content-Type header, got headers: %v", w.Header())
	}
}

func TestFaviconFound(t *testing.T) {
	req := httptest.NewRequest("GET", "/favicon.ico", nil)
	w := httptest.NewRecorder()
	testServer.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected code %d, got %d", http.StatusOK, w.Code)
	}
	if hdr := w.Header().Get("Content-Type"); hdr != "image/x-icon" {
		t.Errorf("incorrect Content-Type header, got headers: %v", w.Header())
	}
}

func TestHSTSHeaderSet(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()
	testServer.ServeHTTP(w, req)
	if hdr := w.Header().Get("Strict-Transport-Security"); hdr == "" {
		t.Errorf("missing Strict-Transport-Security header; headers = %v", w.Header())
	}
}

func TestRandomHelpWantedIssue(t *testing.T) {
	req := httptest.NewRequest("GET", "/imfeelinglucky", nil)
	w := httptest.NewRecorder()
	testServer.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Errorf("w.Code = %d; want %d", w.Code, http.StatusSeeOther)
	}
	if g, w := w.Header().Get("Location"), issuesURLBase; g != w {
		t.Errorf("Location header = %q; want %q", g, w)
	}

	testServer.cMu.Lock()
	testServer.helpWantedIssues = []issueData{{id: 42}}
	testServer.cMu.Unlock()
	w = httptest.NewRecorder()
	testServer.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Errorf("w.Code = %d; want %d", w.Code, http.StatusSeeOther)
	}
	if g, w := w.Header().Get("Location"), issuesURLBase+"42"; g != w {
		t.Errorf("Location header = %q; want %q", g, w)
	}
}

func TestRandomFilteredHelpWantedIssue(t *testing.T) {
	req := httptest.NewRequest("GET", "/imfeelinglucky?pkg=foo", nil)
	w := httptest.NewRecorder()
	testServer.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Errorf("w.Code = %d; want %d", w.Code, http.StatusSeeOther)
	}
	if g, w := w.Header().Get("Location"), issuesURLBase; g != w {
		t.Errorf("Location header = %q; want %q", g, w)
	}

	testServer.cMu.Lock()
	testServer.helpWantedIssues = []issueData{
		{id: 41, titlePrefix: "not/foo: bar"},
		{id: 42, titlePrefix: "foo/bar: baz"},
	}
	testServer.cMu.Unlock()
	w = httptest.NewRecorder()
	testServer.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Errorf("w.Code = %d; want %d", w.Code, http.StatusSeeOther)
	}
	if g, w := w.Header().Get("Location"), issuesURLBase+"42"; g != w {
		t.Errorf("Location header = %q; want %q", g, w)
	}
}

func TestHandleDirRedirect(t *testing.T) {
	tests := []struct {
		path string
		ref  string
		want string
	}{
		{"/dir/build", "", "https://github.com/golang/build/tree/master/"},
		{"/dir/build/", "", "https://github.com/golang/build/tree/master/"},
		{"/dir/build", "https://go.googlesource.com/", "https://go.googlesource.com/build/+/master/"},
		{"/dir/build/", "https://go.googlesource.com/", "https://go.googlesource.com/build/+/master/"},
		{"/dir/build/maintner", "", "https://github.com/golang/build/tree/master/maintner"},
		{"/dir/build/maintner/", "", "https://github.com/golang/build/tree/master/maintner"},
		{"/dir/build/maintner", "https://go.googlesource.com/", "https://go.googlesource.com/build/+/master/maintner"},
		{"/dir/build/maintner/", "https://go.googlesource.com/", "https://go.googlesource.com/build/+/master/maintner"},
	}
	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, nil)
		if tt.ref != "" {
			req.Header.Set("Referer", tt.ref)
		}
		w := httptest.NewRecorder()
		testServer.ServeHTTP(w, req)
		if w.Code != http.StatusFound {
			t.Errorf("for %q from %q, got code %d, want %d", tt.path, tt.ref, w.Code, http.StatusFound)
			continue
		}
		got := w.Header().Get("Location")
		if got != tt.want {
			t.Errorf("for %q from %q, got %q; want %q", tt.path, tt.ref, got, tt.want)
		}
	}
}
