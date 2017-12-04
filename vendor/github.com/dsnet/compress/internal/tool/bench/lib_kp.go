// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build !no_lib_kp

package main

import (
	"io"

	"github.com/klauspost/compress/flate"
)

func init() {
	RegisterEncoder(FormatFlate, "kp",
		func(w io.Writer, lvl int) io.WriteCloser {
			zw, err := flate.NewWriter(w, lvl)
			if err != nil {
				panic(err)
			}
			return zw
		})
	RegisterDecoder(FormatFlate, "kp",
		func(r io.Reader) io.ReadCloser {
			return flate.NewReader(r)
		})
}
