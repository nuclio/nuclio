// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package owners

import (
	"bytes"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strings"
)

type Owner struct {
	GitHubUsername string `json:"githubUsername"`
	GerritEmail    string `json:"gerritEmail"`
}

type Entry struct {
	Primary   []Owner `json:"primary"`
	Secondary []Owner `json:"secondary,omitempty"`
}

type Request struct {
	Payload struct {
		Paths []string `json:"paths"`
	} `json:"payload"`
	Version int `json:"v"` // API version
}

type Response struct {
	Payload struct {
		Entries map[string]*Entry `json:"entries"` // paths in request -> Entry
	} `json:"payload"`
	Error string `json:"error,omitempty"`
}

// match takes a path consisting of the repo name and full path of a file or
// directory within that repo and returns the deepest Entry match in the file
// hierarchy for the given resource.
func match(path string) *Entry {
	var deepestPath string
	for p := range entries {
		if strings.HasPrefix(path, p) && len(p) > len(deepestPath) {
			deepestPath = p
		}
	}
	return entries[deepestPath]
}

// Handler takes one or more paths and returns a map of each to a matching
// Entry struct. If no Entry is matched for the path, the value for the key
// is nil.
func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		serveIndex(w, r)
		return
	case "POST":
		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "unable to decode request", http.StatusBadRequest)
			// TODO: increment expvar for monitoring.
			log.Printf("unable to decode owners request: %v", err)
			return
		}

		var resp Response
		resp.Payload.Entries = make(map[string]*Entry)
		for _, p := range req.Payload.Paths {
			resp.Payload.Entries[p] = match(p)
		}
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(resp); err != nil {
			jsonError(w, "unable to encode response", http.StatusInternalServerError)
			// TODO: increment expvar for monitoring.
			log.Printf("unable to encode owners response: %v", err)
			return
		}
		w.Write(buf.Bytes())
	case "OPTIONS":
		// Likely a CORS preflight request; leave resp.Payload empty.
	default:
		jsonError(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}

func jsonError(w http.ResponseWriter, text string, code int) {
	w.WriteHeader(code)
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(Response{Error: text}); err != nil {
		// TODO: increment expvar for monitoring.
		log.Printf("unable to encode error response: %v", err)
		return
	}
	w.Write(buf.Bytes())
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var buf bytes.Buffer
	if err := indexTmpl.Execute(&buf, entries); err != nil {
		log.Printf("unable to execute index template: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

var indexTmpl = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html lang="en">
<title>Go Code Owners</title>
<meta name=viewport content="width=device-width, initial-scale=1">
<style>
* {
	box-sizing: border-box;
	margin: 0;
	padding: 0;
}
body {
	font-family: sans-serif;
	margin: 1rem 1.5rem;
}
.header {
	color: #666;
	font-size: 90%;
	margin-bottom: 1rem;
}
.table-header {
	font-weight: bold;
	position: sticky;
	top: 0;
}
.table-header,
.entry {
	background-color: #fff;
	border-bottom: 1px solid #ddd;
	display: flex;
	flex-wrap: wrap;
	justify-content: space-between;
	margin: .15rem 0;
	padding: .15rem 0;
}
.path,
.primary,
.secondary {
	flex-basis: 33.3%;
}
</style>
<header class="header">
	Alter these entries at
	<a href="https://go.googlesource.com/build/+/master/devapp/owners/"
		target="_blank" rel="noopener">golang.org/x/build/devapp/owners</a>
</header>
<main>
<div class="table-header">
	<span class="path">Path</span>
	<span class="primary">Primaries</span>
	<span class="secondary">Secondaries</span>
</div>
{{range $path, $entry := .}}
	<div class="entry">
		<span class="path">{{$path}}</span>
		<span class="primary">
			{{range .Primary}}
				<a href="https://github.com/{{.GitHubUsername}}" target="_blank" rel="noopener">{{.GitHubUsername}}</a>
			{{end}}
		</span>
		<span class="secondary">
			{{range .Secondary}}
				<a href="https://github.com/{{.GitHubUsername}}" target="_blank" rel="noopener">{{.GitHubUsername}}</a>
			{{end}}
		</span>
	</div>
{{end}}
</main>
`))
