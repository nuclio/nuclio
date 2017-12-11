// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build cgo,!no_cgo_flate

package main

import "github.com/dsnet/compress/internal/cgo/flate"

func init() {
	RegisterEncoder(FormatFlate, "cgo", flate.NewWriter)
	RegisterDecoder(FormatFlate, "cgo", flate.NewReader)
}
