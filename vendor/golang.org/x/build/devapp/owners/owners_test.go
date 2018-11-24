// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package owners

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMatch(t *testing.T) {
	testCases := []struct {
		path  string
		entry *Entry
	}{
		{
			"crypto/chacha20poly1305/chacha20poly1305.go",
			&Entry{
				Primary:   []Owner{filippo},
				Secondary: []Owner{agl},
			},
		},
		{
			"go/src/archive/zip/a.go",
			&Entry{
				Primary:   []Owner{joetsai},
				Secondary: []Owner{bradfitz},
			},
		},
		{
			"go/path/with/no/owners",
			&Entry{
				Primary: []Owner{rsc, iant, bradfitz},
			},
		},
		{
			"nonexistentrepo/foo/bar", nil,
		},
	}
	for _, tc := range testCases {
		matches := match(tc.path)
		if diff := cmp.Diff(matches, tc.entry); diff != "" {
			t.Errorf("%s: owners differ (-got +want)\n%s", tc.path, diff)
		}
	}
}

func TestHandler(t *testing.T) {
	testCases := []struct {
		method  string
		code    int
		paths   []string
		entries map[string]*Entry
	}{
		{"PUT", http.StatusMethodNotAllowed, nil, nil},
		{"OPTIONS", http.StatusOK, nil, nil},
		{
			"POST", http.StatusOK,
			[]string{"nonexistent/path"},
			map[string]*Entry{"nonexistent/path": nil},
		},
		{
			"POST", http.StatusOK,
			[]string{"go/src/archive/zip/a.go"},
			map[string]*Entry{"go/src/archive/zip/a.go": {Primary: []Owner{joetsai}, Secondary: []Owner{bradfitz}}},
		},
		{
			"POST", http.StatusOK,
			[]string{
				"go/src/archive/zip/a.go",
				"go/src/archive/zip/b.go",
			},
			map[string]*Entry{
				"go/src/archive/zip/a.go": {Primary: []Owner{joetsai}, Secondary: []Owner{bradfitz}},
				"go/src/archive/zip/b.go": {Primary: []Owner{joetsai}, Secondary: []Owner{bradfitz}},
			},
		},
		{
			"POST", http.StatusOK,
			[]string{
				"go/src/archive/zip/a.go",
				"crypto/chacha20poly1305/chacha20poly1305.go",
			},
			map[string]*Entry{
				"go/src/archive/zip/a.go":                     {Primary: []Owner{joetsai}, Secondary: []Owner{bradfitz}},
				"crypto/chacha20poly1305/chacha20poly1305.go": {Primary: []Owner{filippo}, Secondary: []Owner{agl}},
			},
		},
	}

	for _, tc := range testCases {
		var buf bytes.Buffer
		if tc.paths != nil {
			var oReq Request
			oReq.Payload.Paths = tc.paths
			if err := json.NewEncoder(&buf).Encode(oReq); err != nil {
				t.Errorf("could not encode request: %v", err)
				continue
			}
		}
		rStr := buf.String()
		if rStr == "" {
			rStr = "<empty>"
		}
		t.Logf("Request: %v", rStr)
		req, err := http.NewRequest(tc.method, "/owners/", &buf)
		if err != nil {
			t.Errorf("http.NewRequest: %v", err)
			continue
		}
		w := httptest.NewRecorder()
		Handler(w, req)
		resp := w.Result()
		if got, want := resp.StatusCode, tc.code; got != want {
			t.Errorf("status code: got %v; want %v", got, want)
		}

		if tc.code != http.StatusOK || tc.method == "OPTIONS" {
			continue
		}
		var oResp Response
		if err := json.NewDecoder(resp.Body).Decode(&oResp); err != nil {
			t.Errorf("json decode: %v", err)
		}
		if oResp.Error != "" {
			t.Errorf("got unexpected error in response: %q", oResp.Error)
		}
		if diff := cmp.Diff(oResp.Payload.Entries, tc.entries); diff != "" {
			t.Errorf("%s: (-got +want)\n%s", tc.method, diff)
		}
	}
}

func TestIndex(t *testing.T) {
	req, err := http.NewRequest("GET", "/owners/", nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}
	w := httptest.NewRecorder()
	Handler(w, req)
	resp := w.Result()
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Errorf("status code: got %v; want %v", got, want)
	}
}

func TestBadRequest(t *testing.T) {
	req, err := http.NewRequest("POST", "/owners/", bytes.NewBufferString("malformed json"))
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}
	w := httptest.NewRecorder()
	Handler(w, req)
	resp := w.Result()
	if got, want := resp.StatusCode, http.StatusBadRequest; got != want {
		t.Errorf("status code: got %v; want %v", got, want)
	}
	if got, want := resp.Header.Get("Content-Type"), "application/json"; got != want {
		t.Errorf("Content-Type: got %q; want %q", got, want)
	}
	var oResp Response
	if err := json.NewDecoder(resp.Body).Decode(&oResp); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}
	if got, want := oResp.Error, "unable to decode request"; got != want {
		t.Errorf("response error text: got %q; want %q", got, want)
	}
}
