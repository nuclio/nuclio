// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package statusserver contains the status server for the golang.org
// properties and build infrastructure.
package statusserver

import (
	"io"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"golang.org/x/build/status"
)

// Handler is the status server handler.
// It expects to be mounted at /status.
type Handler struct {
	mu          sync.Mutex
	items       map[string]*monItem
	sortedItems []string // sorted keys of the items map
}

func NewHandler() *Handler {
	h := &Handler{
		items: map[string]*monItem{
			"pubsubhelper": &monItem{
				goodFor: 10 * time.Minute,
			},
		},
	}
	h.updateSorted()
	return h
}

// Register registers h in mux at /status.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("/status", mux)
}

// requires h.mu is held
func (h *Handler) getItemLocked(name string) *monItem {
	mi, ok := h.items[name]
	if !ok {
		mi = new(monItem)
		mi.goodFor = 10 * time.Minute // default
		h.items[name] = mi
		h.updateSorted()
	}
	return mi
}

// requires h.mu is held
func (h *Handler) updateSorted() {
	h.sortedItems = nil
	for name := range h.items {
		h.sortedItems = append(h.sortedItems, name)
	}
	sort.Strings(h.sortedItems)
}

// monItem is an item that is monitored. It can be monitored
// externally from the server hosting this handler, or it can be
// self-reported, in which case it'll come in via /status/update.
//
// All state is guarded by h.mu.
type monItem struct {
	goodFor time.Duration

	lastUpdate time.Time
	ok         bool
	errText    string
	warn       string
}

func (mi *monItem) updateFromRequest(r *http.Request) {
	// TODO: record state changes over time, so we have a history of the last
	// N transitions.

	mi.lastUpdate = time.Now()
	if s := r.FormValue("state"); s == "ok" {
		mi.ok = true
		mi.errText = ""
	} else {
		mi.ok = false
		mi.errText = s
	}
	mi.warn = r.FormValue("warn")
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/status/update" {
		h.handleUpdate(w, r)
		return
	}
	io.WriteString(w, "ok\n")
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.TLS == nil:
		http.Error(w, "request requires TLS", http.StatusBadRequest)
		return
	case r.Method != "POST":
		http.Error(w, "request requires POST", http.StatusBadRequest)
		return
	case r.Header.Get("Content-Type") != "application/x-www-form-urlencoded":
		http.Error(w, "request requires Content-Type x-www-form-urlencoded", http.StatusBadRequest)
		return
	case r.ContentLength > 0 && r.ContentLength <= 1<<20:
		http.Error(w, "request requires explicit Content-Length under 1MB", http.StatusBadRequest)
		return
	}
	wantToken, err := status.UpdateToken()
	if err != nil {
		log.Printf("error: status.UpdateToken: %v", err)
		http.Error(w, "failed to get status update token to validate against", 500)
		return
	}
	if r.Header.Get("X-Status-Update-Token") != wantToken {
		http.Error(w, "invalid X-Status-Update-Token value", http.StatusUnauthorized)
		return
	}
	name := r.FormValue("name")

	h.mu.Lock()
	defer h.mu.Unlock()
	mi := h.getItemLocked(name)
	mi.updateFromRequest(r)
	w.WriteHeader(http.StatusNoContent)
}
