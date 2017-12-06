// Copyright 2017, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build gofuzz

package bzip2

import (
	"bytes"
	"io/ioutil"

	gbrotli "github.com/dsnet/compress/brotli"
	cbrotli "github.com/dsnet/compress/internal/cgo/brotli"
)

func Fuzz(data []byte) int {
	// Decompress using the Go decoder.
	gr, err := gbrotli.NewReader(bytes.NewReader(data), nil)
	if err != nil {
		panic(err)
	}
	gb, gerr := ioutil.ReadAll(gr)
	if err := gr.Close(); gerr == nil {
		gerr = err
	} else if gerr != nil && err == nil {
		panic("nil on Close after non-nil error")
	}

	// Decompress using the C decoder.
	cr := cbrotli.NewReader(bytes.NewReader(data))
	cb, cerr := ioutil.ReadAll(cr)
	if err := cr.Close(); cerr == nil {
		cerr = err
	} else if cerr != nil && err == nil {
		panic("nil on Close after non-nil error")
	}

	switch {
	case gerr == nil && cerr == nil:
		if !bytes.Equal(gb, cb) {
			panic("mismatching bytes")
		}
	case gerr != nil && cerr == nil:
		panic(gerr)
	case gerr == nil && cerr != nil:
		panic(cerr)
	default:
		// Ensure that both gb and cb have the same common prefix.
		if !bytes.HasPrefix(gb, cb) && !bytes.HasPrefix(cb, gb) {
			panic("mismatching leading bytes")
		}
	}

	if cerr == nil || gerr == nil {
		return 1 // Favor valid inputs
	}
	return 0
}
