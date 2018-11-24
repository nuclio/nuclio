// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log"
	"net/http/httptest"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"

	"golang.org/x/build/buildlet"
	"golang.org/x/build/dashboard"
)

type TestBuildletPool struct {
	clients map[string]*buildlet.Client
	mu      sync.Mutex
}

// GetBuildlet finds the first available buildlet for the hostType and returns
// it, or an error if no buildlets are available for that hostType.
func (tp *TestBuildletPool) GetBuildlet(ctx context.Context, hostType string, lg logger) (*buildlet.Client, error) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	c, ok := tp.clients[hostType]
	if ok {
		return c, nil
	}
	return nil, fmt.Errorf("No client found for host type %s", hostType)
}

// Add sets the given client for the given hostType, overriding any previous
// entries.
func (tp *TestBuildletPool) Add(hostType string, client *buildlet.Client) {
	tp.mu.Lock()
	if tp.clients == nil {
		tp.clients = make(map[string]*buildlet.Client)
	}
	tp.clients[hostType] = client
	tp.mu.Unlock()
}

func (tp *TestBuildletPool) Remove(hostType string) {
	tp.mu.Lock()
	delete(tp.clients, hostType)
	tp.mu.Unlock()
}

func (tp *TestBuildletPool) String() string { return "test" }

func (tp *TestBuildletPool) HasCapacity(string) bool { return true }

var testPool = &TestBuildletPool{}

func TestHandleBuildletCreateWrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/buildlet/create", nil)
	w := httptest.NewRecorder()
	handleBuildletCreate(w, req)
	if w.Code != 400 {
		t.Fatalf("GET /buildlet/create: expected code 400, got %d", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "POST required") {
		t.Fatalf("GET /buildlet/create: expected 'POST required' error, got %s", body)
	}
}

func TestHandleBuildletCreateOldVersion(t *testing.T) {
	data := url.Values{}
	data.Set("version", "20150922")
	req := httptest.NewRequest("POST", "/buildlet/create", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handleBuildletCreate(w, req)
	if w.Code != 400 {
		t.Fatalf("GET /buildlet/create: expected code 400, got %d", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, `client version "20150922" is too old`) {
		t.Fatalf("GET /buildlet/create: expected 'version too old' error, got %s", body)
	}
}

func addBuilder(name string) {
	dashboard.Builders[name] = &dashboard.BuildConfig{
		Name:     name,
		HostType: "test-host",
		Notes:    "Dummy client for testing",
	}
	dashboard.Hosts["test-host"] = &dashboard.HostConfig{
		HostType: "test-host",
		Owner:    "test@golang.org",
	}
	testPool.Add("test-host", &buildlet.Client{})
}

func removeBuilder(name string) {
	delete(dashboard.Builders, name)
	delete(dashboard.Builders, "test-host")
	testPool.Remove("test-host")
}

var buildName = runtime.GOOS + "-" + runtime.GOARCH + "-test"

type tlogger struct{ t *testing.T }

func (t tlogger) Write(p []byte) (int, error) {
	t.t.Logf("LOG: %s", p)
	return len(p), nil
}

func TestHandleBuildletCreate(t *testing.T) {
	log.SetOutput(tlogger{t})
	defer log.SetOutput(os.Stderr)
	addBuilder(buildName)
	testPoolHook = func(_ *dashboard.BuildConfig) BuildletPool { return testPool }
	defer func() {
		removeBuilder(buildName)
		testPoolHook = nil
	}()
	data := url.Values{}
	data.Set("version", "20160922")
	data.Set("builderType", buildName)
	req := httptest.NewRequest("POST", "/buildlet/create", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handleBuildletCreate(w, req)
	if w.Code != 200 {
		t.Fatal("bad code", w.Code, w.Body.String())
	}
}
