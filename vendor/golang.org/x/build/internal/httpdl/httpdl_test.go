// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package httpdl

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDownload(t *testing.T) {
	defer resetHooks()

	someTime := time.Unix(1462292149, 0)
	const someContent = "this is some content"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "foo.txt", someTime, strings.NewReader(someContent))
	}))
	defer ts.Close()

	tmpDir, err := ioutil.TempDir("", "dl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	dstFile := filepath.Join(tmpDir, "foo.txt")

	hitCurPath := false
	hookIsCurrent = func() { hitCurPath = true }

	// First.
	if err := Download(dstFile, ts.URL+"/foo.txt"); err != nil {
		t.Fatal(err)
	}
	if hitCurPath {
		t.Fatal("first should've actually downloaded")
	}
	if fi, err := os.Stat(dstFile); err != nil {
		t.Fatal(err)
	} else if !fi.ModTime().Equal(someTime) {
		t.Fatalf("modtime = %v; want %v", fi.ModTime(), someTime)
	} else if fi.Size() != int64(len(someContent)) {
		t.Fatalf("size = %v; want %v", fi.Size(), len(someContent))
	}

	// Second.
	if err := Download(dstFile, ts.URL+"/foo.txt"); err != nil {
		t.Fatal(err)
	}
	if !hitCurPath {
		t.Fatal("second shouldn't have downloaded")
	}

	// Then touch to invalidate.
	os.Chtimes(dstFile, time.Now(), time.Now())
	hitCurPath = false // reset
	if err := Download(dstFile, ts.URL+"/foo.txt"); err != nil {
		t.Fatal(err)
	}
	if hitCurPath {
		t.Fatal("should've re-downloaded after modtime change")
	}

	// Also check re-download on size change.
	ioutil.WriteFile(dstFile, []byte(someContent+someContent), 0644)
	os.Chtimes(dstFile, someTime, someTime)
	if err := Download(dstFile, ts.URL+"/foo.txt"); err != nil {
		t.Fatal(err)
	}
	if hitCurPath {
		t.Fatal("should've re-downloaded after size change")
	}
}
