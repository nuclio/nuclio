// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package httpdl downloads things from HTTP to local disk.
package httpdl

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Test hooks:
var (
	hookIsCurrent func()
	// TODO(bradfitz): more?
)

func resetHooks() {
	hookIsCurrent = func() {}
}

func init() {
	resetHooks()
}

// Download downloads url to the named local file.
//
// It stops after a HEAD request if the local file's modtime and size
// look correct.
func Download(file, url string) error {
	// Special case hack to recognize GCS URLs and append a
	// timestamp as a cache buster...
	if strings.HasPrefix(url, "https://storage.googleapis.com") && !strings.Contains(url, "?") {
		url += fmt.Sprintf("?%d", time.Now().Unix())
	}

	if res, err := head(url); err != nil {
		return err
	} else if diskFileIsCurrent(file, res) {
		hookIsCurrent()
		return nil
	}

	res, err := http.Get(url)
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("HTTP status code of %s was %v", url, res.Status)
	}
	modStr := res.Header.Get("Last-Modified")
	modTime, err := http.ParseTime(modStr)
	if err != nil {
		return fmt.Errorf("invalid or missing Last-Modified header %q: %v", modStr, err)
	}
	tmp := file + ".tmp"
	os.Remove(tmp)
	os.Remove(file)
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, res.Body)
	res.Body.Close()
	if err != nil {
		return fmt.Errorf("error copying %v to %v: %v", url, file, err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chtimes(tmp, modTime, modTime); err != nil {
		return err
	}
	if err := os.Rename(tmp, file); err != nil {
		return err
	}
	return nil
}

func head(url string) (*http.Response, error) {
	res, err := http.Head(url)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP response of %s was %v (after HEAD request)", url, res.Status)
	}
	return res, nil
}

func diskFileIsCurrent(file string, res *http.Response) bool {
	fi, err := os.Stat(file)
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	mod := res.Header.Get("Last-Modified")
	clen := res.Header.Get("Content-Length")
	if mod == "" || clen == "" {
		return false
	}
	clen64, err := strconv.ParseInt(clen, 10, 64)
	if err != nil || clen64 != fi.Size() {
		return false
	}
	modTime, err := http.ParseTime(mod)
	return err == nil && modTime.Equal(fi.ModTime())
}
