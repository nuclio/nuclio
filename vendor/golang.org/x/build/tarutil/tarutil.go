// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tarutil contains utilities for working with tar archives.
package tarutil

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
)

// FileList is a list of entries in a tar archive which acts
// as a template to make .tar.gz io.Readers as needed.
//
// The zero value is a valid empty list.
//
// All entries must be added before calling OpenTarGz.
type FileList struct {
	files []headerContent
}

type headerContent struct {
	header *tar.Header

	// For regular files:
	size    int64
	content io.ReaderAt
}

// AddHeader adds a non-regular file to the FileList.
func (fl *FileList) AddHeader(h *tar.Header) {
	fl.files = append(fl.files, headerContent{header: h})
}

// AddRegular adds a regular file to the FileList.
func (fl *FileList) AddRegular(h *tar.Header, size int64, content io.ReaderAt) {
	fl.files = append(fl.files, headerContent{
		header:  h,
		size:    size,
		content: content,
	})
}

// TarGz returns an io.ReadCloser of a gzip-compressed tar file
// containing the contents of the FileList.
// All Add calls must happen before OpenTarGz is called.
// Callers must call Close on the returned ReadCloser to release
// resources.
func (fl *FileList) TarGz() io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		err := fl.writeTarGz(pw)
		pw.CloseWithError(err)
	}()
	return struct {
		io.Reader
		io.Closer
	}{
		pr,
		funcCloser(func() error {
			pw.CloseWithError(errors.New("tarutil: .tar.gz generation aborted with Close"))
			return nil
		}),
	}
}

func (fl *FileList) writeTarGz(w *io.PipeWriter) error {
	zw := gzip.NewWriter(w)
	tw := tar.NewWriter(zw)
	for _, f := range fl.files {
		if err := tw.WriteHeader(f.header); err != nil {
			return err
		}
		if f.content != nil {
			if _, err := io.CopyN(tw, io.NewSectionReader(f.content, 0, f.size), f.size); err != nil {
				return err
			}
		}
	}

	if err := tw.Close(); err != nil {
		return err
	}
	return zw.Close()
}

// funcCloser implements io.Closer with a function.
type funcCloser func() error

func (fn funcCloser) Close() error { return fn() }
