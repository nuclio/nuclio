// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/dsnet/compress/internal/testutil"
)

var zcheck = flag.Bool("zcheck", false, "verify test vectors with C bzip2 library")

func cmdCompress(input []byte) ([]byte, error)   { return cmdExec(input, "-z") }
func cmdDecompress(input []byte) ([]byte, error) { return cmdExec(input, "-d") }

// cmdExec executes the bzip2 tool, passing the input in as stdin.
// It returns the stdout and an error.
func cmdExec(input []byte, args ...string) ([]byte, error) {
	var bo, be bytes.Buffer
	cmd := exec.Command("bzip2", args...)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = &bo
	cmd.Stderr = &be
	err := cmd.Run()
	ss := strings.Split(strings.TrimSpace(be.String()), "\n")
	if len(ss) > 0 && ss[len(ss)-1] != "" {
		// Assume any stderr indicates an error and last line is the message.
		return nil, errors.New(ss[len(ss)-1])
	}
	return bo.Bytes(), err
}

var testdata = []struct {
	name  string
	data  []byte
	ratio float64 // The minimum expected ratio (uncompressed / compressed)
}{
	{"Nil", nil, 0},
	{"Binary", testutil.MustLoadFile("../testdata/binary.bin"), 5.68},
	{"Digits", testutil.MustLoadFile("../testdata/digits.txt"), 2.22},
	{"Huffman", testutil.MustLoadFile("../testdata/huffman.txt"), 1.24},
	{"Random", testutil.MustLoadFile("../testdata/random.bin"), 0.98},
	{"Repeats", testutil.MustLoadFile("../testdata/repeats.bin"), 3.93},
	{"Twain", testutil.MustLoadFile("../testdata/twain.txt"), 2.99},
	{"Zeros", testutil.MustLoadFile("../testdata/zeros.bin"), 5825.0},
}

var levels = []struct {
	name  string
	level int
}{
	{"Speed", BestSpeed},
	{"Default", DefaultCompression},
	{"Compression", BestCompression},
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
	for _, v := range testdata {
		v := v
		t.Run(v.name, func(t *testing.T) {
			t.Parallel()

			var buf1, buf2 bytes.Buffer

			// Compress the input.
			wr, err := NewWriter(&buf1, nil)
			if err != nil {
				t.Errorf("NewWriter() = (_, %v), want (_, nil)", err)
			}
			n, err := io.Copy(wr, bytes.NewReader(v.data))
			if n != int64(len(v.data)) || err != nil {
				t.Errorf("Copy() = (%d, %v), want (%d, nil)", n, err, len(v.data))
			}
			if err := wr.Close(); err != nil {
				t.Errorf("Close() = %v, want nil", err)
			}

			// Verify that the compression ratio is within expected bounds.
			ratio := float64(len(v.data)) / float64(buf1.Len())
			if ratio < v.ratio {
				t.Errorf("poor compression ratio: %0.2f < %0.2f", ratio, v.ratio)
			}

			// Verify that the C library can decompress the output of Writer and
			// that the Reader can decompress the output of the C library.
			if *zcheck {
				zd, err := cmdDecompress(buf1.Bytes())
				if err != nil {
					t.Errorf("unexpected cmdDecompress error: %v", err)
				}
				if got, want, ok := testutil.BytesCompare(zd, v.data); !ok {
					t.Errorf("output data mismatch:\ngot  %s\nwant %s", got, want)
				}
				zc, err := cmdCompress(v.data)
				if err != nil {
					t.Errorf("unexpected cmdCompress error: %v", err)
				}
				zratio := float64(len(v.data)) / float64(len(zc))
				if ratio < 0.9*zratio {
					t.Errorf("poor compression ratio: %0.2f < %0.2f", ratio, 0.9*zratio)
				}
				buf1.Reset()
				buf1.Write(zc) // Use output of C library for Reader test
			}

			// Decompress the output.
			rd, err := NewReader(&buf1, nil)
			if err != nil {
				t.Errorf("NewReader() = (_, %v), want (_, nil)", err)
			}
			n, err = io.Copy(&buf2, rd)
			if n != int64(len(v.data)) || err != nil {
				t.Errorf("Copy() = (%d, %v), want (%d, nil)", n, err, len(v.data))
			}
			if err := rd.Close(); err != nil {
				t.Errorf("Close() = %v, want nil", err)
			}
			if got, want, ok := testutil.BytesCompare(buf2.Bytes(), v.data); !ok {
				t.Errorf("output data mismatch:\ngot  %s\nwant %s", got, want)
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
