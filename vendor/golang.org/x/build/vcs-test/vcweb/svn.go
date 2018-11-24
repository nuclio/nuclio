// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Subversion is so complicated it can only run from inside Apache.
// Assume an appropriately configured Apache is on 8888.
func svnHandler() http.Handler {
	u, err := url.Parse("http://127.0.0.1:8888/")
	if err != nil {
		log.Fatal(err)
	}
	return httputil.NewSingleHostReverseProxy(u)
}
