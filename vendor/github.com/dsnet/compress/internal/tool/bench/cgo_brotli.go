// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build cgo,!no_cgo_brotli

package main

import "github.com/dsnet/compress/internal/cgo/brotli"

func init() {
	RegisterEncoder(FormatBrotli, "cgo", brotli.NewWriter)
	RegisterDecoder(FormatBrotli, "cgo", brotli.NewReader)
}
