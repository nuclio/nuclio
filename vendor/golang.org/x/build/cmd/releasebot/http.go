// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

func newLogger(t http.RoundTripper) http.RoundTripper {
	return &loggingTransport{transport: t}
}

type loggingTransport struct {
	transport http.RoundTripper
	mu        sync.Mutex
	active    []byte
}

func (t *loggingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.mu.Lock()
	index := len(t.active)
	start := time.Now()
	fmt.Fprintf(os.Stderr, "HTTP: %s %s+ %s\n", timeFormat1(start), t.active, r.URL)
	t.active = append(t.active, '|')
	t.mu.Unlock()

	resp, err := t.transport.RoundTrip(r)

	last := r.URL.Path
	if i := strings.LastIndex(last, "/"); i >= 0 {
		last = last[i:]
	}
	display := last
	if resp != nil {
		display += " " + resp.Status
	}
	if err != nil {
		display += " error: " + err.Error()
	}
	now := time.Now()

	t.mu.Lock()
	t.active[index] = '-'
	fmt.Fprintf(os.Stderr, "HTTP: %s %s %s (%.3fs)\n", timeFormat1(now), t.active, display, now.Sub(start).Seconds())
	t.active[index] = ' '
	n := len(t.active)
	for n%4 == 0 && n >= 4 && t.active[n-1] == ' ' && t.active[n-2] == ' ' && t.active[n-3] == ' ' && t.active[n-4] == ' ' {
		t.active = t.active[:n-4]
		n -= 4
	}
	t.mu.Unlock()

	return resp, err
}

func timeFormat1(t time.Time) string {
	return t.Format("15:04:05.000")
}
