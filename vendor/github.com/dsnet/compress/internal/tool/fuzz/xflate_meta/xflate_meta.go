// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build gofuzz

package xflate_meta

import (
	"bytes"
	"compress/flate"
	"io/ioutil"

	"github.com/dsnet/compress/xflate"
)

func Fuzz(data []byte) int {
	mdata, ok := decodeMeta(data)
	if ok {
		testRoundTrip(mdata)
		return 1
	} else {
		testRoundTrip(data)
		return 0
	}
}

// decodeMeta attempts to decode the metadata.
// If successful, it verifies that meta-encoded blocks are DEFLATE blocks.
func decodeMeta(data []byte) ([]byte, bool) {
	r := bytes.NewReader(data)
	mr := xflate.NewMetaReader(r)
	b, err := ioutil.ReadAll(mr)
	if err != nil {
		return nil, false
	}
	pos := int(r.Size()) - r.Len()
	decompressMeta(data[:pos])
	return b, true
}

// decompressMeta attempts to decompress the meta-encoded blocks.
// It expects decompression to succeed and to output nothing.
func decompressMeta(data []byte) {
	// Make a copy and append DEFLATE terminator block.
	data = append([]byte(nil), data...)
	data = append(data, []byte{0x01, 0x00, 0x00, 0xff, 0xff}...)

	r := bytes.NewReader(data)
	for r.Len() > 0 {
		zr := flate.NewReader(r)
		b, err := ioutil.ReadAll(zr)
		if err != nil {
			panic(err)
		}
		if len(b) > 0 {
			panic("non-zero meta-encoded block")
		}
		if err := zr.Close(); err != nil {
			panic(err)
		}
	}
}

// testRoundTrip encodes the input data and then decodes it, checking that the
// metadata was losslessly preserved.
func testRoundTrip(want []byte) {
	bb := new(bytes.Buffer)
	mw := xflate.NewMetaWriter(bb)
	n, err := mw.Write(want)
	if n != len(want) || err != nil {
		panic(err)
	}
	if err := mw.Close(); err != nil {
		panic(err)
	}

	got, ok := decodeMeta(bb.Bytes())
	if !bytes.Equal(got, want) || !ok {
		panic("mismatching bytes")
	}
}
