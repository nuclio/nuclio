// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

import "io"

type bitWriter struct {
	wr     io.Writer
	offset int64 // Number of bytes written to underlying io.Writer
}

func (bw *bitWriter) Init(w io.Writer) {
	return
}

func (bw *bitWriter) Write(buf []byte) (int, error) {
	return 0, nil
}

func (bw *bitWriter) WriteBits(val, nb uint) {
	return
}

func (bw *bitWriter) WritePads() {
	return
}

func (bw *bitWriter) WriteSymbol(pe *prefixEncoder, sym uint) {
	return
}
