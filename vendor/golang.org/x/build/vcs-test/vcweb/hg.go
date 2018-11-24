// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cgi"
	"os"
	"path/filepath"
	"strings"
)

var hgwebPy = `#!/usr/bin/python
from mercurial import demandimport; demandimport.enable()
from mercurial.hgweb import hgweb, wsgicgi
wsgicgi.launch(hgweb("../hgweb.cfg"))
`

var hgwebCfg = `
[paths]
/hg/ = /DIR/hg/*
`

func hgHandler() http.Handler {
	py := filepath.Join(*dir, "hgweb.py")
	if err := ioutil.WriteFile(py, []byte(hgwebPy), 0777); err != nil {
		log.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(*dir, "hgweb.cfg"), []byte(strings.Replace(hgwebCfg, "DIR", *dir, -1)), 0777); err != nil {
		log.Fatal(err)
	}
	os.Mkdir(filepath.Join(*dir, "hg"), 0777)

	return &cgi.Handler{
		Path: py,
		Dir:  filepath.Join(*dir, "hg"),
	}
}
