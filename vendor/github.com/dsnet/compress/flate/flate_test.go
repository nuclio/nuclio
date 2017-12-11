// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package flate

import (
	"bufio"
	"bytes"
	"io"
	"testing"

	// TODO(dsnet): We should not be relying on the standard library for the
	// round-trip test.
	"compress/flate"

	"github.com/dsnet/compress/internal/testutil"
)

var testdata = []struct {
	name string
	data []byte
}{
	{"Nil", nil},
	{"Binary", testutil.MustLoadFile("../testdata/binary.bin")},
	{"Digits", testutil.MustLoadFile("../testdata/digits.txt")},
	{"Huffman", testutil.MustLoadFile("../testdata/huffman.txt")},
	{"Random", testutil.MustLoadFile("../testdata/random.bin")},
	{"Repeats", testutil.MustLoadFile("../testdata/repeats.bin")},
	{"Twain", testutil.MustLoadFile("../testdata/twain.txt")},
	{"Zeros", testutil.MustLoadFile("../testdata/zeros.bin")},
}

var levels = []struct {
	name  string
	level int
}{
	{"Huffman", flate.HuffmanOnly},
	{"Speed", flate.BestSpeed},
	{"Default", flate.DefaultCompression},
	{"Compression", flate.BestCompression},
}

var sizes = []struct {
	name string
	size int
}{
	{"1e4", 1e4},
	{"1e5", 1e5},
	{"1e6", 1e6},
}

func TestRoundTrip(t *testing.T) {
	for i, v := range testdata {
		var buf1, buf2 bytes.Buffer

		// Compress the input.
		wr, err := flate.NewWriter(&buf1, flate.DefaultCompression)
		if err != nil {
			t.Errorf("test %d, NewWriter() = (_, %v), want (_, nil)", i, err)
		}
		n, err := io.Copy(wr, bytes.NewReader(v.data))
		if n != int64(len(v.data)) || err != nil {
			t.Errorf("test %d, Copy() = (%d, %v), want (%d, nil)", i, n, err, len(v.data))
		}
		if err := wr.Close(); err != nil {
			t.Errorf("test %d, Close() = %v, want nil", i, err)
		}

		// Write a canary byte to ensure this does not get read.
		buf1.WriteByte(0x7a)

		// Decompress the output.
		rd, err := NewReader(&buf1, nil)
		if err != nil {
			t.Errorf("test %d, NewReader() = (_, %v), want (_, nil)", i, err)
		}
		n, err = io.Copy(&buf2, rd)
		if n != int64(len(v.data)) || err != nil {
			t.Errorf("test %d, Copy() = (%d, %v), want (%d, nil)", i, n, err, len(v.data))
		}
		if err := rd.Close(); err != nil {
			t.Errorf("test %d, Close() = %v, want nil", i, err)
		}
		if got, want, ok := testutil.BytesCompare(buf2.Bytes(), v.data); !ok {
			t.Errorf("test %d, output data mismatch:\ngot  %s\nwant %s", i, got, want)
		}

		// Read back the canary byte.
		if v, _ := buf1.ReadByte(); v != 0x7a {
			t.Errorf("Read consumed more data than necessary")
		}
	}
}

// syncBuffer is a special reader that records whether the Reader ever tried to
// read past the io.EOF. Since the flate Writer and Reader should be in sync,
// the reader should never attempt to read past the sync marker, otherwise the
// reader could potentially end up blocking on a network read when it had enough
// data to report back to the user.
type syncBuffer struct {
	bytes.Buffer
	blocked bool // blocked reports where a Read would have blocked
}

func (sb *syncBuffer) Read(buf []byte) (int, error) {
	n, err := sb.Buffer.Read(buf)
	if n == 0 && len(buf) > 0 {
		sb.blocked = true
	}
	return n, err
}

func (sb *syncBuffer) ReadByte() (byte, error) {
	b, err := sb.Buffer.ReadByte()
	if err == io.EOF {
		sb.blocked = true
	}
	return b, err
}

// TestSync tests that the Reader can read all data compressed thus far by the
// Writer once Flush is called.
func TestSync(t *testing.T) {
	const prime = 13
	var flushSizes []int
	for i := 1; i < 100; i += 3 {
		flushSizes = append(flushSizes, i)
	}
	for i := 1; i <= 1<<16; i *= 4 {
		flushSizes = append(flushSizes, i)
		flushSizes = append(flushSizes, i+prime)
	}
	for i := 1; i <= 10000; i *= 10 {
		flushSizes = append(flushSizes, i)
		flushSizes = append(flushSizes, i+prime)
	}

	// Load test data of sufficient size.
	var maxSize, totalSize int
	for _, n := range flushSizes {
		totalSize += n
		if maxSize < n {
			maxSize = n
		}
	}
	maxBuf := make([]byte, maxSize)
	data := testutil.MustLoadFile("../testdata/twain.txt")
	data = testutil.ResizeData(data, totalSize)

	for _, name := range []string{"Reader", "ByteReader", "BufferedReader"} {
		t.Run(name, func(t *testing.T) {
			data := data // Closure to ensure fresh data per iteration

			// Test each type of reader.
			var rdBuf io.Reader
			buf := new(syncBuffer)
			switch name {
			case "Reader":
				rdBuf = struct{ io.Reader }{buf}
			case "ByteReader":
				rdBuf = buf // syncBuffer already has a ReadByte method
			case "BufferedReader":
				rdBuf = bufio.NewReader(buf)
			default:
				t.Errorf("unknown reader type: %s", name)
				return
			}

			wr, _ := flate.NewWriter(buf, flate.DefaultCompression)
			rd, err := NewReader(rdBuf, nil)
			if err != nil {
				t.Errorf("unexpected NewReader error: %v", err)
			}
			for _, n := range flushSizes {
				// Write and flush some portion of the test data.
				want := data[:n]
				data = data[n:]
				if _, err := wr.Write(want); err != nil {
					t.Errorf("flushSize: %d, unexpected Write error: %v", n, err)
				}
				if err := wr.Flush(); err != nil {
					t.Errorf("flushSize: %d, unexpected Flush error: %v", n, err)
				}

				// Verify that we can read all data flushed so far.
				m, err := io.ReadAtLeast(rd, maxBuf, n)
				if err != nil {
					t.Errorf("flushSize: %d, unexpected ReadAtLeast error: %v", n, err)
				}
				got := maxBuf[:m]
				if got, want, ok := testutil.BytesCompare(got, want); !ok {
					t.Errorf("flushSize: %d, output mismatch:\ngot  %s\nwant %s", n, got, want)
				}
				if buf.Len() > 0 {
					t.Errorf("flushSize: %d, unconsumed buffer data: %d bytes", n, buf.Len())
				}
				if buf.blocked {
					t.Errorf("flushSize: %d, attempted over-consumption of buffer", n)
				}
				buf.blocked = false
			}
		})
	}
}

func runBenchmarks(b *testing.B, f func(b *testing.B, buf []byte, lvl int)) {
	for _, td := range testdata {
		if len(td.data) == 0 {
			continue
		}
		if testing.Short() && !(td.name == "Twain" || td.name == "Digits") {
			continue
		}
		for _, tl := range levels {
			for _, ts := range sizes {
				buf := testutil.ResizeData(td.data, ts.size)
				b.Run(td.name+"/"+tl.name+"/"+ts.name, func(b *testing.B) {
					f(b, buf, tl.level)
				})
			}
		}
	}
}
