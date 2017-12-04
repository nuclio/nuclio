// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package xflate

import (
	"bytes"
	"io"
	"testing"

	"github.com/dsnet/compress/internal/testutil"
)

var (
	testBinary  = testutil.MustLoadFile("../testdata/binary.bin")
	testDigits  = testutil.MustLoadFile("../testdata/digits.txt")
	testHuffman = testutil.MustLoadFile("../testdata/huffman.txt")
	testRandom  = testutil.MustLoadFile("../testdata/random.bin")
	testRepeats = testutil.MustLoadFile("../testdata/repeats.bin")
	testTwain   = testutil.MustLoadFile("../testdata/twain.txt")
	testZeros   = testutil.MustLoadFile("../testdata/zeros.bin")
)

func TestRoundTrip(t *testing.T) {
	vectors := []struct {
		name  string
		input []byte
	}{
		{"Nil", nil},
		{"Binary", testBinary},
		{"Digits", testDigits},
		{"Huffman", testHuffman},
		{"Random", testRandom},
		{"Repeats", testRepeats},
		{"Twain", testTwain},
		{"Zeros", testZeros},
	}

	for _, v := range vectors {
		v := v
		t.Run(v.name, func(t *testing.T) {
			t.Parallel()

			var wb, rb bytes.Buffer

			xw, err := NewWriter(&wb, &WriterConfig{ChunkSize: 1 << 10})
			if err != nil {
				t.Errorf("unexpected error: NewWriter() = %v", err)
			}
			cnt, err := io.Copy(xw, bytes.NewReader(v.input))
			if err != nil {
				t.Errorf("unexpected error: Write() = %v", err)
			}
			if cnt != int64(len(v.input)) {
				t.Errorf("write count mismatch: got %d, want %d", cnt, len(v.input))
			}
			if err := xw.Close(); err != nil {
				t.Errorf("unexpected error: Close() = %v", err)
			}

			xr, err := NewReader(bytes.NewReader(wb.Bytes()), nil)
			if err != nil {
				t.Errorf("unexpected error: NewReader() = %v", err)
			}
			cnt, err = io.Copy(&rb, xr)
			if err != nil {
				t.Errorf("unexpected error: Read() = %v", err)
			}
			if cnt != int64(len(v.input)) {
				t.Errorf("read count mismatch: got %d, want %d", cnt, len(v.input))
			}
			if err := xr.Close(); err != nil {
				t.Errorf("unexpected error: Close() = %v", err)
			}

			output := rb.Bytes()
			if got, want, ok := testutil.BytesCompare(output, v.input); !ok {
				t.Errorf("output data mismatch:\ngot  %s\nwant %s", got, want)
			}
		})
	}
}
