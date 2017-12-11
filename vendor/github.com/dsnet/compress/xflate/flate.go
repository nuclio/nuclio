// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package xflate

import (
	"bufio"
	"compress/flate"
	"io"
)

// TODO(dsnet): The standard library's version of flate.Reader and flate.Writer
// do not track the input and output offsets. When we eventually switch over
// to using the DEFLATE implementation in this repository, we can delete these.

// countReader is a trivial io.Reader that counts the number of bytes read.
type countReader struct {
	R io.Reader
	N int64
}

func (cr *countReader) Read(buf []byte) (int, error) {
	n, err := cr.R.Read(buf)
	cr.N += int64(n)
	return n, err
}

// flateReader is a trivial wrapper around flate.Reader keeps tracks of offsets.
type flateReader struct {
	InputOffset  int64 // Total number of bytes read from underlying io.Reader
	OutputOffset int64 // Total number of bytes emitted from Read

	zr io.ReadCloser
	br *bufio.Reader
	cr countReader
}

func newFlateReader(rd io.Reader) (*flateReader, error) {
	fr := new(flateReader)
	fr.cr = countReader{R: rd}
	fr.br = bufio.NewReader(&fr.cr)
	fr.zr = flate.NewReader(fr.br)
	return fr, nil
}

func (fr *flateReader) Reset(rd io.Reader) {
	*fr = flateReader{zr: fr.zr, br: fr.br}
	fr.cr = countReader{R: rd}
	fr.br.Reset(&fr.cr)
	fr.zr.(flate.Resetter).Reset(fr.br, nil)
}

func (fr *flateReader) Read(buf []byte) (int, error) {
	offset := fr.cr.N - int64(fr.br.Buffered())
	n, err := fr.zr.Read(buf)
	fr.InputOffset += (fr.cr.N - int64(fr.br.Buffered())) - offset
	fr.OutputOffset += int64(n)
	return n, errWrap(err)
}

// countWriter is a trivial io.Writer that counts the number of bytes written.
type countWriter struct {
	W io.Writer
	N int64
}

func (cw *countWriter) Write(buf []byte) (int, error) {
	n, err := cw.W.Write(buf)
	cw.N += int64(n)
	return n, err
}

// flateWriter is a trivial wrapper around flate.Writer keeps tracks of offsets.
type flateWriter struct {
	InputOffset  int64 // Total number of bytes issued to Write
	OutputOffset int64 // Total number of bytes written to underlying io.Writer

	zw *flate.Writer
	cw countWriter
}

func newFlateWriter(wr io.Writer, lvl int) (*flateWriter, error) {
	var err error
	fw := new(flateWriter)
	switch lvl {
	case 0:
		lvl = flate.DefaultCompression
	case -1:
		lvl = flate.NoCompression
	}
	fw.cw = countWriter{W: wr}
	fw.zw, err = flate.NewWriter(&fw.cw, lvl)
	return fw, errWrap(err)
}

func (fw *flateWriter) Reset(wr io.Writer) {
	*fw = flateWriter{zw: fw.zw}
	fw.cw = countWriter{W: wr}
	fw.zw.Reset(&fw.cw)
}

func (fw *flateWriter) Write(buf []byte) (int, error) {
	offset := fw.cw.N
	n, err := fw.zw.Write(buf)
	fw.OutputOffset += fw.cw.N - offset
	fw.InputOffset += int64(n)
	return n, errWrap(err)
}

func (fw *flateWriter) Flush() error {
	offset := fw.cw.N
	err := fw.zw.Flush()
	fw.OutputOffset += fw.cw.N - offset
	return errWrap(err)
}
