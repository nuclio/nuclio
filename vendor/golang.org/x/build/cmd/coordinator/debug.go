// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"text/template"

	"golang.org/x/build/internal/buildgo"
	"golang.org/x/build/types"
)

// handleDoSomeWork adds the last committed CL as work to do.
//
// Only available in dev mode.
func handleDoSomeWork(work chan<- buildgo.BuilderRev) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			buf := new(bytes.Buffer)
			if err := tmplDoSomeWork.Execute(buf, reversePool.HostTypes()); err != nil {
				http.Error(w, fmt.Sprintf("dosomework: %v", err), http.StatusInternalServerError)
			}
			buf.WriteTo(w)
			return
		}
		if r.Method != "POST" {
			http.Error(w, "dosomework only takes GET and POST", http.StatusBadRequest)
			return
		}

		mode := strings.TrimPrefix(r.URL.Path, "/dosomework/")

		count, err := strconv.Atoi(r.FormValue("count"))
		if err != nil {
			count = 1
		}

		// Cap number of jobs that can be scheduled from debug UI. If
		// buildEnv.MaxBuilds is zero, there is no cap.
		if buildEnv.MaxBuilds > 0 && count > buildEnv.MaxBuilds {
			count = buildEnv.MaxBuilds
		}
		log.Printf("looking for %v work items for %q", count, mode)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "looking for work for %s...\n", mode)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		revs, err := latestBuildableGoRev(count)
		if err != nil {
			fmt.Fprintf(w, "cannot find revision: %v", err)
			return
		}
		fmt.Fprintf(w, "found work: %v\n", revs)
		for _, rev := range revs {
			work <- buildgo.BuilderRev{Name: mode, Rev: rev}
		}
	}
}

// latestBuildableGoRev returns the specified number of most recent buildable
// revisions. If there are not enough buildable revisions available to satisfy
// the specified amount, unbuildable revisions will be used to meet the
// specified count.
func latestBuildableGoRev(count int) ([]string, error) {
	var bs types.BuildStatus
	var revisions []string
	res, err := http.Get("https://build.golang.org/?mode=json")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&bs); err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected build.golang.org http status %v", res.Status)
	}
	// Find first count "ok" revisions
	for _, br := range bs.Revisions {
		if br.Repo == "go" {
			for _, res := range br.Results {
				if res == "ok" {
					revisions = append(revisions, br.Revision)
					break
				}
			}
		}
		if len(revisions) == count {
			return revisions, nil
		}
	}

	// If there weren't enough "ok" revisions, add enough "not ok"
	// revisions to satisfy count.
	for _, br := range bs.Revisions {
		if br.Repo == "go" {
			revisions = append(revisions, br.Revision)
			if len(revisions) == count {
				return revisions, nil
			}
		}
	}
	return nil, errors.New("no revisions on build.golang.org")
}

var tmplDoSomeWork = template.Must(template.New("").Parse(`
<html><head><title>do some work</title></head><body>
<h1>do some work</h1>
{{range .}}
<form action="/dosomework/{{.}}" method="POST"><button>{{.}}</button></form><br\>
{{end}}
<form action="/dosomework/linux-amd64-kube" method="POST"><input type="text" name="count" id="count" value="1"></input><button>linux-amd64-kube</button></form><br\>
</body></html>
`))
