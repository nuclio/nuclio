// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/dsnet/compress/internal/testutil"
)

func TestRunLengthEncoder(t *testing.T) {
	vectors := []struct {
		size   int
		input  string
		output string
		done   bool
	}{{
		size:   0,
		input:  "",
		output: "",
	}, {
		size:   6,
		input:  "abc",
		output: "abc",
	}, {
		size:   6,
		input:  "abcccc",
		output: "abccc",
		done:   true,
	}, {
		size:   7,
		input:  "abcccc",
		output: "abcccc\x00",
	}, {
		size:   14,
		input:  "aaaabbbbcccc",
		output: "aaaa\x00bbbb\x00ccc",
		done:   true,
	}, {
		size:   15,
		input:  "aaaabbbbcccc",
		output: "aaaa\x00bbbb\x00cccc\x00",
	}, {
		size:   16,
		input:  strings.Repeat("a", 4),
		output: "aaaa\x00",
	}, {
		size:   16,
		input:  strings.Repeat("a", 255),
		output: "aaaa\xfb",
	}, {
		size:   16,
		input:  strings.Repeat("a", 256),
		output: "aaaa\xfba",
	}, {
		size:   16,
		input:  strings.Repeat("a", 259),
		output: "aaaa\xfbaaaa\x00",
	}, {
		size:   16,
		input:  strings.Repeat("a", 500),
		output: "aaaa\xfbaaaa\xf1",
	}, {
		size:   64,
		input:  "aaabbbcccddddddeeefgghiiijkllmmmmmmmmnnoo",
		output: "aaabbbcccdddd\x02eeefgghiiijkllmmmm\x04nnoo",
	}}

	buf := make([]byte, 3)
	for i, v := range vectors {
		rd := strings.NewReader(v.input)
		rle := new(runLengthEncoding)
		rle.Init(make([]byte, v.size))
		_, err := io.CopyBuffer(rle, struct{ io.Reader }{rd}, buf)
		output := rle.Bytes()

		if got, want, ok := testutil.BytesCompare(output, []byte(v.output)); !ok {
			t.Errorf("test %d, output mismatch:\ngot  %s\nwant %s", i, got, want)
		}
		if done := err == rleDone; done != v.done {
			t.Errorf("test %d, done mismatch: got %v want %v", i, done, v.done)
		}
	}
}

func TestRunLengthDecoder(t *testing.T) {
	vectors := []struct {
		input  string
		output string
		fail   bool
	}{{
		input:  "",
		output: "",
	}, {
		input:  "abc",
		output: "abc",
	}, {
		input:  "aaaa",
		output: "aaaa",
		fail:   true,
	}, {
		input:  "baaaa\x00aaaa",
		output: "baaaaaaaa",
		fail:   true,
	}, {
		input:  "abcccc\x00",
		output: "abcccc",
	}, {
		input:  "aaaa\x00bbbb\x00ccc",
		output: "aaaabbbbccc",
	}, {
		input:  "aaaa\x00bbbb\x00cccc\x00",
		output: "aaaabbbbcccc",
	}, {
		input:  "aaaa\x00aaaa\x00aaaa\x00",
		output: "aaaaaaaaaaaa",
	}, {
		input:  "aaaa\xffaaaa\xffaaaa\xff",
		output: strings.Repeat("a", 259*3),
	}, {
		input:  "bbbaaaa\xffaaaa\xffaaaa\xff",
		output: "bbb" + strings.Repeat("a", 259*3),
	}, {
		input:  "aaaa\x00",
		output: strings.Repeat("a", 4),
	}, {
		input:  "aaaa\xfb",
		output: strings.Repeat("a", 255),
	}, {
		input:  "aaaa\xfba",
		output: strings.Repeat("a", 256),
	}, {
		input:  "aaaa\xfbaaaa\x00",
		output: strings.Repeat("a", 259),
	}, {
		input:  "aaaa\xfbaaaa\xf1",
		output: strings.Repeat("a", 500),
	}, {
		input:  "aaabbbcccdddd\x02eeefgghiiijkllmmmm\x04nnoo",
		output: "aaabbbcccddddddeeefgghiiijkllmmmmmmmmnnoo",
	}}

	buf := make([]byte, 3)
	for i, v := range vectors {
		wr := new(bytes.Buffer)
		rle := new(runLengthEncoding)
		rle.Init([]byte(v.input))
		_, err := io.CopyBuffer(struct{ io.Writer }{wr}, rle, buf)
		output := wr.Bytes()

		if got, want, ok := testutil.BytesCompare(output, []byte(v.output)); !ok {
			t.Errorf("test %d, output mismatch:\ngot  %s\nwant %s", i, got, want)
		}
		if fail := err != rleDone; fail != v.fail {
			t.Errorf("test %d, failure mismatch: got %t, want %t", i, fail, v.fail)
		}
	}
}
