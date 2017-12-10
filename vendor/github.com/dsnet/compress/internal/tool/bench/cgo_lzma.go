// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build cgo,!no_cgo_lzma

package main

import "github.com/dsnet/compress/internal/cgo/lzma"

func init() {
	RegisterEncoder(FormatLZMA2, "cgo", lzma.NewWriter)
	RegisterDecoder(FormatLZMA2, "cgo", lzma.NewReader)
}
