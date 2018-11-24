// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"net/http"
	"net/http/cgi"
	"os"
	"os/exec"
	"path/filepath"
)

func gitHandler() http.Handler {
	os.Mkdir(filepath.Join(*dir, "git"), 0777)
	path, err := exec.LookPath("git")
	if err != nil {
		log.Fatal(err)
	}
	return &cgi.Handler{
		Path: path,
		Args: []string{"http-backend"},
		Dir:  filepath.Join(*dir, "git"),
		Env: []string{
			"GIT_PROJECT_ROOT=" + filepath.Join(*dir),
			"GIT_HTTP_EXPORT_ALL=1",
		},
	}
}
