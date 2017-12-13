// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build gofuzz

package bzip2_bwt

import (
	"bytes"
	"hash/adler32"

	"github.com/dsnet/compress/bzip2"
)

func Fuzz(data []byte) int {
	if len(data) == 0 {
		return -1
	}
	testReverse(data)
	testRoundTrip(data)
	return 0
}

// testReverse verifies that we can reverse the BWT on any arbitrary input
// so long as we choose a valid origin pointer.
func testReverse(data []byte) {
	data = append([]byte(nil), data...) // Make copy of data
	ptr := int(adler32.Checksum(data)) % len(data)
	bzip2.ReverseBWT(data, ptr)
}

// testRoundTrip verifies that a round-trip BWT faithfully reproduces the
// input data set.
func testRoundTrip(want []byte) {
	got := append([]byte(nil), want...)
	ptr := bzip2.ForwardBWT(got)
	bzip2.ReverseBWT(got, ptr)

	if ptr < 0 || ptr >= len(want) {
		panic("invalid origin pointer")
	}
	if !bytes.Equal(got, want) {
		panic("mismatching bytes")
	}
}
