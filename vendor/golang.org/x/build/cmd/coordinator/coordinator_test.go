// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/build/internal/buildgo"
)

func TestPartitionGoTests(t *testing.T) {
	var in []string
	for name := range fixedTestDuration {
		in = append(in, name)
	}
	sets := partitionGoTests("", in)
	for i, set := range sets {
		t.Logf("set %d = \"-run=^(%s)$\"", i, strings.Join(set, "|"))
	}
}

func TestTryStatusJSON(t *testing.T) {
	testCases := []struct {
		desc   string
		method string
		ts     *trySet
		tss    trySetState
		status int
		body   string
	}{
		{
			"pre-flight CORS header",
			"OPTIONS",
			nil,
			trySetState{},
			http.StatusOK,
			``,
		},
		{
			"nil trySet",
			"GET",
			nil,
			trySetState{},
			http.StatusNotFound,
			`{"success":false,"error":"TryBot result not found (already done, invalid, or not yet discovered from Gerrit). Check Gerrit for results."}` + "\n",
		},
		{"non-nil trySet",
			"GET",
			&trySet{
				tryKey: tryKey{
					Commit:   "deadbeef",
					ChangeID: "Ifoo",
				},
			},
			trySetState{
				builds: []*buildStatus{
					{
						BuilderRev: buildgo.BuilderRev{Name: "linux"},
						startTime:  time.Time{}.Add(24 * time.Hour),
						done:       time.Time{}.Add(48 * time.Hour),
						succeeded:  true,
					},
					{
						BuilderRev: buildgo.BuilderRev{Name: "macOS"},
						startTime:  time.Time{}.Add(24 * time.Hour),
					},
				},
			},
			http.StatusOK,
			`{"success":true,"payload":{"changeId":"Ifoo","commit":"deadbeef","builds":[{"name":"linux","startTime":"0001-01-02T00:00:00Z","done":true,"succeeded":true},{"name":"macOS","startTime":"0001-01-02T00:00:00Z","done":false,"succeeded":false}]}}` + "\n"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			w := httptest.NewRecorder()
			r, err := http.NewRequest(tc.method, "", nil)
			if err != nil {
				t.Fatalf("could not create http.Request: %v", err)
			}
			serveTryStatusJSON(w, r, tc.ts, tc.tss)
			resp := w.Result()
			hdr := "Access-Control-Allow-Origin"
			if got, want := resp.Header.Get(hdr), "*"; got != want {
				t.Errorf("unexpected %q header: got %q; want %q", hdr, got, want)
			}
			if got, want := resp.StatusCode, tc.status; got != want {
				t.Errorf("response status code: got %d; want %d", got, want)
			}
			defer resp.Body.Close()
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("ioutil.ReadAll: %v", err)
			}
			if got, want := string(b), tc.body; got != want {
				t.Errorf("body: got\n%v\nwant\n%v", got, want)
			}
		})
	}
}

func TestStagingClusterBuilders(t *testing.T) {
	// Just test that it doesn't panic:
	stagingClusterBuilders()
}
