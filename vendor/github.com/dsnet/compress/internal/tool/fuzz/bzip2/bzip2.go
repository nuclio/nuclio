// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build gofuzz

package bzip2

import (
	"bytes"
	"errors"
	"io/ioutil"

	"github.com/dsnet/compress"
	gbzip2 "github.com/dsnet/compress/bzip2"
	cbzip2 "github.com/dsnet/compress/internal/cgo/bzip2"
)

func Fuzz(data []byte) int {
	data, ok := testDecoders(data, true)
	for i := 1; i <= 9; i++ {
		testGoEncoder(data, i)
		testCEncoder(data, i)
	}
	if ok {
		return 1 // Favor valid inputs
	}
	return 0
}

// testDecoders tests that the input can be handled by both Go and C decoders.
// This test does not panic if both decoders run into an error, since it
// means that they both agree that the input is bad.
//
// If updateCRCs is set, then the Go bzip2 implementation will ignore all
// checksum errors and manually adjust the checksum values before running the
// C implementation. This hack drastically increases the probability that
// gofuzz can generate a "valid" file.
func testDecoders(data []byte, updateCRCs bool) ([]byte, bool) {
	// Decompress using the Go decoder.
	gr, err := gbzip2.NewReader(bytes.NewReader(data), nil)
	if err != nil {
		panic(err)
	}
	gb, gerr := ioutil.ReadAll(gr)
	if err := gr.Close(); gerr == nil {
		gerr = err
	} else if gerr != nil && err == nil {
		panic("nil on Close after non-nil error")
	}

	// Check or update the checksums.
	if gerr == nil {
		if updateCRCs {
			data = gr.Checksums.Apply(data)
		} else if !gr.Checksums.Verify(data) {
			gerr = errors.New("bzip2: checksum error")
		}
	}

	// Decompress using the C decoder.
	cr := cbzip2.NewReader(bytes.NewReader(data))
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
		return gb, true
	case gerr != nil && cerr == nil:
		// Ignore deprecated errors since there are no plans to provide
		// these features in the Go implementation.
		if err, ok := gerr.(compress.Error); ok && err.IsDeprecated() {
			return cb, false
		}
		panic(gerr)
	case gerr == nil && cerr != nil:
		panic(cerr)
	default:
		// Ensure that both gb and cb have the same common prefix.
		if !bytes.HasPrefix(gb, cb) && !bytes.HasPrefix(cb, gb) {
			panic("mismatching leading bytes")
		}
		return nil, false
	}
}

// testGoEncoder encodes the input data with the Go encoder and then checks that
// both the Go and C decoders can properly decompress the output.
func testGoEncoder(data []byte, level int) {
	// Compress using the Go encoder.
	bb := new(bytes.Buffer)
	gw, err := gbzip2.NewWriter(bb, &gbzip2.WriterConfig{Level: level})
	if err != nil {
		panic(err)
	}
	defer gw.Close()
	n, err := gw.Write(data)
	if n != len(data) || err != nil {
		panic(err)
	}
	if err := gw.Close(); err != nil {
		panic(err)
	}

	// Decompress using both the Go and C decoders.
	b, ok := testDecoders(bb.Bytes(), false)
	if !ok {
		panic("decoder error")
	}
	if !bytes.Equal(b, data) {
		panic("mismatching bytes")
	}
}

// testCEncoder encodes the input data with the C encoder and then checks that
// both the Go and C decoders can properly decompress the output.
func testCEncoder(data []byte, level int) {
	// Compress using the C encoder.
	bb := new(bytes.Buffer)
	cw := cbzip2.NewWriter(bb, level)
	defer cw.Close()
	n, err := cw.Write(data)
	if n != len(data) || err != nil {
		panic(err)
	}
	if err := cw.Close(); err != nil {
		panic(err)
	}

	// Decompress using both the Go and C decoders.
	b, ok := testDecoders(bb.Bytes(), false)
	if !ok {
		panic("decoder error")
	}
	if !bytes.Equal(b, data) {
		panic("mismatching bytes")
	}
}
