// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tarutil

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
	"time"
)

// fileInfo is an os.FileInfo implementation for tarHeader.
type fileInfo struct {
	name   string
	mode   os.FileMode
	size   int64
	target string // if symlink
}

func (fi fileInfo) IsDir() bool        { return fi.mode.IsDir() }
func (fi fileInfo) ModTime() time.Time { return time.Time{} }
func (fi fileInfo) Mode() os.FileMode  { return fi.mode }
func (fi fileInfo) Name() string       { return path.Base(fi.name) }
func (fi fileInfo) Size() int64        { return fi.size }
func (fi fileInfo) Sys() interface{}   { return nil }

func tarHeader(t *testing.T, fi fileInfo) *tar.Header {
	h, err := tar.FileInfoHeader(fi, fi.target)
	if err != nil {
		t.Fatalf("tarHeader: %v", err)
	}
	h.Name = fi.name // see docs on tar.FileInfoHeader
	return h
}

func TestFileList(t *testing.T) {
	fl := new(FileList)

	fl.AddHeader(tarHeader(t, fileInfo{name: "symlink-file", target: "link-target", mode: 0644 | os.ModeSymlink}))
	fl.AddRegular(tarHeader(t, fileInfo{name: "regular.txt", mode: 0644, size: 7}), 7, strings.NewReader("foo bar"))

	tgz := fl.TarGz()
	defer tgz.Close()
	zr, err := gzip.NewReader(tgz)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	tr := tar.NewReader(zr)
	saw := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Reader.Next: %v", err)
		}
		saw++
		switch h.Name {
		case "symlink-file":
		case "regular.txt":
			all, err := ioutil.ReadAll(tr)
			if err != nil {
				t.Fatalf("Reading regular.txt: %v", err)
			}
			if string(all) != "foo bar" {
				t.Errorf("regular.txt = %q; want \"foo bar\"", all)
			}
		default:
			t.Fatalf("unknown name %q", h.Name)
		}
	}
	if saw != 2 {
		t.Errorf("number of entries = %d; want 2", saw)
	}
}
