// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io/ioutil"
	"net/http"
	"net/http/cgi"
	"os"
	"path/filepath"
	"strings"
)

func fossilHandler() http.Handler {
	return http.HandlerFunc(fossilDispatch)
}

func fossilDispatch(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/fossil/") {
		w.WriteHeader(404)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/fossil/")
	if i := strings.Index(name, "/"); i >= 0 {
		name = name[:i]
	}

	db := filepath.Join(*dir, "fossil/"+name+"/"+name+".fossil")
	_, err := os.Stat(db)
	if err != nil {
		w.WriteHeader(404)
	}
	if _, err := os.Stat(db + ".cgi"); err != nil {
		ioutil.WriteFile(db+".cgi", []byte("#!/usr/bin/fossil\nrepository: "+db+"\n"), 0777)
	}

	h := &cgi.Handler{
		Path: db + ".cgi",
		Root: "/fossil/" + name,
		Dir:  filepath.Join(*dir, "fossil"),
	}
	h.ServeHTTP(w, r)
}
