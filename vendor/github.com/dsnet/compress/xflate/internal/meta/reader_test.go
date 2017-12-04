// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package meta

import (
	"bytes"
	"io"
	"io/ioutil"
	"math/rand"
	"strings"
	"testing"

	"github.com/dsnet/compress/internal/errors"
	"github.com/dsnet/compress/internal/testutil"
)

// TestReader tests that the reader is able to properly decode a set of valid
// input strings or properly detect corruption in a set of invalid input
// strings. A third-party decoder should verify that it has the same behavior
// when processing these input vectors.
func TestReader(t *testing.T) {
	db := testutil.MustDecodeBitGen
	dh := testutil.MustDecodeHex

	errFuncs := map[string]func(error) bool{
		"IsEOF":           func(err error) bool { return err == io.EOF },
		"IsUnexpectedEOF": func(err error) bool { return err == io.ErrUnexpectedEOF },
		"IsCorrupted":     errors.IsCorrupted,
	}
	vectors := []struct {
		desc   string    // Description of the test
		input  []byte    // Test input string
		output []byte    // Expected output string
		final  FinalMode // Expected FinalMode value
		errf   string    // Name of error checking callback
	}{{
		desc:   "empty string",
		input:  dh(""),
		output: dh(""),
		errf:   "IsEOF",
	}, {
		desc: "bad empty meta block (FinalNil, first symbol not symZero)",
		input: db(`<<<
			< (0 10) (00100 00000 1010) (011 000 011 001 000 (000 000)*4 010)
			> (111 <D7:127) (111 <D7:100) 10 (110 <D2:3) 10
			< 0*4 0 1*3
		`),
		output: dh(""),
		errf:   "IsCorrupted",
	}, {
		desc: "empty meta block (FinalNil)",
		input: db(`<<<
			< (0 10) (00011 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> (111 <D7:127) (111 <D7:99) 10 (110 <D2:3) 10
			< 0*3 0 1*3
		`),
		output: dh(""),
		final:  FinalNil,
	}, {
		desc: "empty meta block (FinalMeta)",
		input: db(`<<<
			< (0 10) (00011 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> 10 (111 <D7:127) (111 <D7:99) 10 (110 <D2:3)
			< 0*3 0 1*3
		`),
		output: dh(""),
		final:  FinalMeta,
	}, {
		desc: "bad empty meta block, contains the magic value mid way",
		input: db(`<<<
			< (1 10) (00000 00000 1100) (011 000 011 001 000 (000 000)*5 010) 0
			> 10 0*14 10 0*13 (110 <D2:0) 0 (110 <D2:1) 0*4 (111 <D7:127)
			  (111 <D7:59) 0*5 10*2
			< 0*0 0 1*2
		`),
		output: dh(""),
		errf:   "IsCorrupted",
	}, {
		desc: "meta block containing the string 'a'",
		input: db(`<<<
			< (0 10) (00010 00000 1000) (011 000 011 001 000 (000 000)*3 010) 0
			> 10 0 10 0*4 10 0*4 10*2 (111 <D7:127) (111 <D7:82) 10 (110 <D2:3)
			  (110 <D2:1)
			< 0*2 0 1*4
		`),
		output: []byte("a"),
		final:  FinalMeta,
	}, {
		desc: "meta block containing the string 'ab'",
		input: db(`<<<
			< (0 10) (00010 00000 1000) (011 000 011 001 000 (000 000)*3 010) 0
			> 10 0*2 10 0*3 10 0*4 10*2 0*2 10 0*3 10*2 (111 <D7:127)
			  (111 <D7:77) 10 (110 <D2:3) 10
			< 0*2 0 1*4
		`),
		output: []byte("ab"),
		final:  FinalMeta,
	}, {
		desc: "meta block containing the string 'abc'",
		input: db(`<<<
			< (0 10) (00010 00000 0110) (011 000 011 001 000 (000 000)*2 010) 0
			> 10 0 10*2 0*3 10 0*4 10*2 0*2 10 0*3 10*2 0 10*2 0*3 10*2
			  (111 <D7:127) (111 <D7:58) 10 (110 <D2:3) (110 <D2:3) (110 <D2:3)
			< 0*2 0 1*5
		`),
		output: []byte("abc"),
		final:  FinalMeta,
	}, {
		desc: "meta block containing the string 'Hello, world!'",
		input: db(`<<<
			< (0 10) (00010 00000 0100) (011 000 011 001 000 (000 000)*1 010) 0
			> 10 0 10 0 10*2 0*4 10 0*2 10 0 10 0 10 0*2 10*2 0*3 10*2 0 10*2
			  0*3 10*2 0 10*2 0 10 (110 <D2:0) 0 10*2 0*3 10*2 0 10 0
			  (110 <D2:3) 10 0*2 10*3 0 10*3 0 10 (110 <D2:0) 0 10*2 0*2 10 0*2
			  10*3 0*3 10*2 0 10*2 0*3 10 0*2 10*2 0 10 0*4 10 (111 <D7:125)
			  10 (110 <D2:3) (110 <D2:1)
			< 0*2 0 1*6
		`),
		output: []byte("Hello, world!"),
		final:  FinalMeta,
	}, {
		desc: "meta block containing the hex-string '00'*4",
		input: db(`<<<
			< (0 10) (00110 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> 10 0*3 10 (111 <D7:127) (111 <D7:96) 10 (110 <D2:2)
			< 0*6 0 1*3
		`),
		output: dh("00000000"),
		final:  FinalMeta,
	}, {
		desc: "meta block containing the hex-string '00'*8",
		input: db(`<<<
			< (0 10) (00101 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> 10 0*4 10 (111 <D7:127) (111 <D7:95) 10 (110 <D2:2)
			< 0*5 0 1*3
		`),
		output: dh("0000000000000000"),
		final:  FinalMeta,
	}, {
		desc: "meta block containing the hex-string '00'*16",
		input: db(`<<<
			< (0 10) (00100 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> 10 0*5 10 (111 <D7:127) (111 <D7:94) 10 (110 <D2:2)
			< 0*4 0 1*3
		`),
		output: dh("00000000000000000000000000000000"),
		final:  FinalMeta,
	}, {
		desc: "meta block containing the hex-string 'ff'*4",
		input: db(`<<<
			< (0 10) (00101 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> 10*2 0*2 10 (111 <D7:127) (111 <D7:97) 10 (110 <D2:1)
			< 0*5 0 1*3
		`),
		output: dh("ffffffff"),
		final:  FinalMeta,
	}, {
		desc: "meta block containing the hex-string 'ff'*8",
		input: db(`<<<
			< (0 10) (00100 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> 10*2 0*3 10 (111 <D7:127) (111 <D7:96) 10 (110 <D2:1)
			< 0*4 0 1*3
		`),
		output: dh("ffffffffffffffff"),
		final:  FinalMeta,
	}, {
		desc: "meta block containing the hex-string 'ff'*16",
		input: db(`<<<
			< (0 10) (00011 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> 10*2 0*4 10 (111 <D7:127) (111 <D7:95) 10 (110 <D2:1)
			< 0*3 0 1*3
		`),
		output: dh("ffffffffffffffffffffffffffffffff"),
		final:  FinalMeta,
	}, {
		desc: "meta block containing the random hex-string '911fe47084a4668b'",
		input: db(`<<<
			< (0 10) (00011 00000 0100) (011 000 011 001 000 (000 000) 010) 0
			> 10 0*4 10 0 10 0*3 10 0*2 10 (110 <D2:2) 0*5 10 0*2 10*3 0*4 10*3
			  0*3 10 0*4 10 0*2 10 0*2 10 0 10 0 10*2 0*2 10*2 0 10*2 0 10 0*3
			  10 (111 <D7:127) (111 <D7:2) 10 (110 <D2:3)*5 (110 <D2:0)
			< 0*3 0 1*6
		`),
		output: dh("911fe47084a4668b"),
		final:  FinalMeta,
	}, {
		desc: "meta block containing the random hex-string 'de9fa94cb16f40fc'",
		input: db(`<<<
			< (0 10) (00100 00000 0100) (011 000 011 001 000 (000 000) 010) 0
			> 10*2 0*3 10 0 10 0*4 10 0 (110 <D2:3) 10*2 0*2 10*2 0 10 0 10 0
			  10*2 0*2 10*2 0 10 0 10*2 10 0*2 10 0*5 10 0*2 10 (110 <D2:3) 0
			  10*3 (111 <D7:127) (111 <D7:9) 10 (110 <D2:3)*5 10*2
			< 0*4 0 1*6
		`),
		output: dh("de9fa94cb16f40fc"),
		final:  FinalMeta,
	}, {
		desc: "empty meta block with a huffLen of 1",
		input: db(`<<<
			< (0 10) (00110 00000 1110) (011 000 011 001 000 (000 000)*6 010) 0
			> 10 (111 <D7:127) (111 <D7:105) 10
			< 0*6 0 1*1
		`),
		output: dh(""),
		final:  FinalMeta,
	}, {
		desc: "empty meta block with a huffLen of 2",
		input: db(`<<<
			< (0 10) (00111 00000 1100) (011 000 011 001 000 (000 000)*5 010) 0
			> 10 (111 <D7:127) 10*2 (111 <D7:103) 10
			< 0*7 0 1*2
		`),
		output: dh(""),
		final:  FinalMeta,
	}, {
		desc: "empty meta block with a huffLen of 3",
		input: db(`<<<
			< (0 10) (00100 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> 10 (111 <D7:127) 10*6 (111 <D7:99) 10
			< 0*4 0 1*3
		`),
		output: dh(""),
		final:  FinalMeta,
	}, {
		desc: "empty meta block with a huffLen of 4",
		input: db(`<<<
			< (0 10) (00001 00000 1000) (011 000 011 001 000 (000 000)*3 010) 0
			> 10 (111 <D7:127) 10*14 (111 <D7:91) 10
			< 0*1 0 1*4
		`),
		output: dh(""),
		final:  FinalMeta,
	}, {
		desc: "empty meta block with a huffLen of 5",
		input: db(`<<<
			< (0 10) (00110 00000 0110) (011 000 011 001 000 (000 000)*2 010) 0
			> 10 (111 <D7:127) 10*30 (111 <D7:75) 10
			< 0*6 0 1*5
		`),
		output: dh(""),
		final:  FinalMeta,
	}, {
		desc: "empty meta block with a huffLen of 6",
		input: db(`<<<
			< (0 10) (00011 00000 0100) (011 000 011 001 000 (000 000)*1 010) 0
			> 10 (111 <D7:127) 10*62 (111 <D7:43) 10
			< 0*3 0 1*6
		`),
		output: dh(""),
		final:  FinalMeta,
	}, {
		desc: "empty meta block with a huffLen of 7",
		input: db(`<<<
			< (0 10) (00010 00000 0010) (011 000 011 001 000 (000 000)*0 010) 0
			> 10 (111 <D7:117) 10*127
			< 0*2 0 1*7
		`),
		output: dh(""),
		final:  FinalMeta,
	}, {
		desc: "shortest meta block",
		input: db(`<<<
			< (0 10) (00011 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> (111 <D7:127) (111 <D7:99) 10 (110 <D2:3) 10
			< 0*3 0 1*3
		`),
		output: dh(""),
	}, {
		desc: "longest meta block",
		input: db(`<<<
			< (0 10) (00000 00000 0010) (011 000 011 001 000 (000 000)*0 010) 0
			> 0*2 (110 <D2:0)*42 10*128
			< 0*0 0 1*7
		`),
		output: dh(""),
	}, {
		desc: "longest decoded meta block",
		input: db(`<<<
			< (0 10) (00100 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> 10*7 (111 <D7:113)*2 10
			< 0*4 0 1*3
		`),
		output: dh("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
		final:  FinalMeta,
	}, {
		desc: "meta block truncated short",
		input: db(`<<<
			< (0 10) (00011 00000 0100) (011 000 011 001 000 (000 000)*1 010) 0
			> 10 0*4 10 0 10 0*3 10 0*2 10 (110 <D2:2) 0*5 10 0*2 10*3 0*4 10*3
			  0*3 10 0*4 10 0*2 10 0*2 10 0 10 0 10*2 0*2 10*2 0 10*2 0 10 0*3
			  10 (111 <D7:127) (111 <D7:2) 10 (110 <D2:3)*5 (110 <D2:0)
			< 0*3 0 1*6
		`)[:3],
		errf: "IsUnexpectedEOF",
	}, {
		desc: "meta block truncated medium-short",
		input: db(`<<<
			< (0 10) (00011 00000 0100) (011 000 011 001 000 (000 000)*1 010) 0
			> 10 0*4 10 0 10 0*3 10 0*2 10 (110 <D2:2) 0*5 10 0*2 10*3 0*4 10*3
			  0*3 10 0*4 10 0*2 10 0*2 10 0 10 0 10*2 0*2 10*2 0 10*2 0 10 0*3
			  10 (111 <D7:127) (111 <D7:2) 10 (110 <D2:3)*5 (110 <D2:0)
			< 0*3 0 1*6
		`)[:4],
		errf: "IsUnexpectedEOF",
	}, {
		desc: "meta block truncated medium-long",
		input: db(`<<<
			< (0 10) (00011 00000 0100) (011 000 011 001 000 (000 000)*1 010) 0
			> 10 0*4 10 0 10 0*3 10 0*2 10 (110 <D2:2) 0*5 10 0*2 10*3 0*4 10*3
			  0*3 10 0*4 10 0*2 10 0*2 10 0 10 0 10*2 0*2 10*2 0 10*2 0 10 0*3
			  10 (111 <D7:127) (111 <D7:2) 10 (110 <D2:3)*5 (110 <D2:0)
			< 0*3 0 1*6
		`)[:13],
		errf: "IsUnexpectedEOF",
	}, {
		desc: "meta block truncated long",
		input: db(`<<<
			< (0 10) (00011 00000 0100) (011 000 011 001 000 (000 000)*1 010) 0
			> 10 0*4 10 0 10 0*3 10 0*2 10 (110 <D2:2) 0*5 10 0*2 10*3 0*4 10*3
			  0*3 10 0*4 10 0*2 10 0*2 10 0 10 0 10*2 0*2 10*2 0 10*2 0 10 0*3
			  10 (111 <D7:127) (111 <D7:2) 10 (110 <D2:3)*5 (110 <D2:0)
			< 0*3 0 1*6
		`)[:24],
		errf: "IsUnexpectedEOF",
	}, {
		desc:  "random junk",
		input: dh("911fe47084a4668b"),
		errf:  "IsCorrupted",
	}, {
		desc: "meta block with invalid number of HCLen codes of 6",
		input: db(`<<<
			< (0 10) (00110 00000 0000) (011 000 011 001 000 (000 000)*0 000)
			> 0*34 10 0 10 (111 <D7:127) (111 <D7:105)
			< 000001 0 100
		`),
		errf: "IsCorrupted",
	}, {
		desc: "meta block with invalid HCLen code in the middle",
		input: db(`<<<
			< (0 10) (00110 00000 1110) (011 000 011 001 000 (000 010) (000 000)*5 010) 0
			> 10 (111 <D7:127) (111 <D7:105) 10
			< 000000 0 1
		`),
		errf: "IsCorrupted",
	}, {
		desc: "meta block with invalid HCLen code at the end",
		input: db(`<<<
			< (0 10) (00110 00000 1110) (011 000 011 001 000 (000 000)*6 110) 0
			> 10 (111 <D7:127) (111 <D7:105) 10
			< 000000 0 1
		`),
		errf: "IsCorrupted",
	}, {
		desc: "meta block first symbol being a last repeater",
		input: db(`<<<
			< (0 10) (00100 00000 1110) (011 000 011 001 000 (000 000)*6 010)
			> (110 <D2:0) 10 (111 <D7:127) (111 <D7:104)
			< 0000 0 1
		`),
		errf: "IsCorrupted",
	}, {
		desc: "meta block with too many symbols",
		input: db(`<<<
			< (0 10) (00110 00000 1110) (011 000 011 001 000 (000 000)*6 010) 0
			> 10 (111 <D7:127) (111 <D7:106) 10
			< 000000 0 1
		`),
		errf: "IsCorrupted",
	}, {
		desc:  "meta block with too few symbols",
		input: dh("34c087050000000020fe7f3a40"),
		errf:  "IsCorrupted",
	}, {
		desc: "meta block with first symbol not a zero",
		input: db(`<<<
			< (0 10) (00110 00000 1110) (011 000 011 001 000 (000 000)*6 010) 0
			> 10 (111 <D7:127) (111 <D7:104) 10
			< 000000 0 0
		`),
		errf: "IsCorrupted",
	}, {
		desc: "meta block with no EOB symbol",
		input: db(`<<<
			< (0 10) (00101 00000 1110) (011 000 011 001 000 (000 000)*6 010) 0
			> 10 (111 <D7:127) (111 <D7:104) 10 0
			< 00000 0 1
		`),
		errf: "IsCorrupted",
	}, {
		desc: "meta block with FinalStream set, but not FinalMeta",
		input: db(`<<<
			< (1 10) (00101 00000 1110) (011 000 011 001 000 (000 000)*6 010) 0
			> 0 10 (111 <D7:127) (111 <D7:104) 10
			< 00000 0 1
		`),
		errf: "IsCorrupted",
	}, {
		desc: "meta block with some padding bits not zero",
		input: db(`<<<
			< (0 10) (00110 00000 1110) (011 000 011 001 000 (000 000)*6 010) 0
			> 10 (111 <D7:127) (111 <D7:105) 10
			< 100000 0 1
		`),
		errf: "IsCorrupted",
	}, {
		desc: "meta block with the HDist tree not empty",
		input: db(`<<<
			< (0 10) (00110 00000 1110) (011 000 011 001 000 (000 000)*6 010) 0
			> 10 (111 <D7:127) (111 <D7:105) 10
			< 000000 1 1
		`),
		errf: "IsCorrupted",
	}, {
		desc: "meta block with invalid EOB",
		input: db(`<<<
			< (0 10) (00110 00000 1110) (011 000 011 001 000 (000 000)*6 010) 0
			> 10 (111 <D7:127) (111 <D7:105) 10
			< 000000 0 0
		`),
		errf: "IsCorrupted",
	}, {
		desc: "meta block with wrong number of padding bits",
		input: db(`<<<
			< (0 10) (00101 00000 1110) (011 000 011 001 000 (000 000)*6 010) 0
			> 10 (111 <D7:127) (111 <D7:105) 10
			< 00000 0 1
		`),
		errf: "IsCorrupted",
	}}

	for i, v := range vectors {
		mr := NewReader(bytes.NewReader(v.input))
		err := mr.decodeBlock()
		output := mr.buf

		if got, want, ok := testutil.BytesCompare(output, v.output); !ok {
			t.Errorf("test %d (%s), mismatching data:\ngot  %s\nwant %s", i, v.desc, got, want)
		}
		if int(mr.InputOffset) != len(v.input) && err == nil {
			t.Errorf("test %d (%s), mismatching offset: got %d, want %d", i, v.desc, mr.InputOffset, len(v.input))
		}
		if mr.final != v.final {
			t.Errorf("test %d (%s), mismatching final mode: got %d, want %d", i, v.desc, mr.final, v.final)
		}
		if v.errf != "" && !errFuncs[v.errf](err) {
			t.Errorf("test %d (%s), mismatching error:\ngot %v\nwant %s(err) == true", i, v.desc, err, v.errf)
		} else if v.errf == "" && err != nil {
			t.Errorf("test %d (%s), unexpected error: got %v", i, v.desc, err)
		}
	}
}

func TestReaderReset(t *testing.T) {
	buf := make([]byte, 512)
	mr := NewReader(nil)

	// Test Reader for idempotent Close.
	if err := mr.Close(); err != nil {
		t.Errorf("unexpected error: Close() = %v", err)
	}
	if err := mr.Close(); err != nil {
		t.Errorf("unexpected error: Close() = %v", err)
	}
	if _, err := mr.Read(buf); err != errClosed {
		t.Errorf("unexpected error: Read() = %v, want %v", err, errClosed)
	}

	// Test Reader with corrupt data.
	mr.Reset(strings.NewReader("corrupt"))
	if _, err := mr.Read(buf); !errors.IsCorrupted(err) {
		t.Errorf("unexpected error: Read() = %v, want IsCorrupted(err) == true", err)
	}
	if err := mr.Close(); !errors.IsCorrupted(err) {
		t.Errorf("unexpected error: Close() = %v, want IsCorrupted(err) == true", err)
	}

	// Test Reader on multiple back-to-back streams.
	data := testutil.MustDecodeBitGen(`<<<
		# FinalNil, "The quick brown fox jumped o"
		< (0 10) (00111 00000 0010) (011 000 011 001 000 010) 0
		> (110 <D2:1) 10*3 0*2 10 0 10 0 10 0 (110 <D2:0) 10 0 10*2 0 10 0 10
		  0*2 10*2 0 (110 <D2:2) 10 0*2 10 0*3 10*3 0 10 0 10 0 10*3 0 10 0*2
		  10 0 10*2 0 10*2 0*3 10*2 0 10*2 0 10 0 10*2 0 (110 <D2:2) 10 0*3 10
		  0*3 10*2 0*2 10 0*2 10*3 0 10 (110 <D2:0) 0 10*2 0 10*3 0 10*3 0*2
		  10*3 0 10*2 0 (110 <D2:2) 10 0*3 10*2 0*2 10*2 0 10 (110 <D2:0) 0 10*2
		  0 (110 <D2:0) 10 (110 <D2:0) 0 (110 <D2:2) 10 0*3 10 0 10 0 10*2 0 10
		  0 10 0 10*3 0 10 0 10*2 0 10*2 0 (110 <D2:1) 10*3 0 10 0 10 0*2 10*2
		  0*3 10 0*2 10*2 0 (110 <D2:2) 10 0*2 10 (110 <D2:0) 0 10*2 0
		  (110 <D2:2) 10 (110 <D2:3) (110 <D2:3) (110 <D2:3) 10
		< 0*7 0 1*7

		# FinalMeta, "ver the lazy dog."
		< (0 10) (00101 00000 0010) (011 000 011 001 000 010) 0
		> 10 0 10 0*3 10 0 10*2 0 10*3 0 10 0 10 0*2 10*2 0*2 10 0*2 10*3 0
		  (110 <D2:2) 10 0 (110 <D2:0) 10 0 10*3 0 (110 <D2:0) 10 0 10*2 0 10 0
		  10 0*2 10*2 0 (110 <D2:2) 10 0 (110 <D2:0) 10*2 0 10*2 0 10 0
		  (110 <D2:0) 10*2 0*2 10 0 10 (110 <D2:0) 0 10 0*2 10 (110 <D2:0) 0
		  (110 <D2:2) 10 0 (110 <D2:0) 10 0*2 10*2 0 10 (110 <D2:0) 0 10*2 0
		  10*3 0*2 10*2 0*2 10*3 0 10 (111 <D7:41) 10 (110 <D2:3) (110 <D2:3)
		  (110 <D2:3) (110 <D2:3) (110 <D2:3) (110 <D2:3) (110 <D2:3)
		  (110 <D2:3) (110 <D2:3) (110 <D2:3) 10*2
		< 0*5 0 1*7

		# FinalNil, "Lorem ipsum dolor sit amet, "
		< (0 10) (00111 00000 0010) (011 000 011 001 000 010) 0
		> (110 <D2:1) 10*3 0*2 10*2 0*2 10 0 10 (110 <D2:0) 0 10*2 0*2 10 0*2
		  10*3 0 10 0 10 0*2 10*2 0 10 0 10*2 0 10*2 0 (110 <D2:2) 10 0*2 10 0*2
		  10 0 10*2 0 (110 <D2:1) 10*3 0 10*2 0*2 10*3 0 10 0 10 0 10*3 0 10 0
		  10*2 0 10*2 0 (110 <D2:2) 10 0 (110 <D2:0) 10 0*2 10*2 0 10
		  (110 <D2:0) 0 10*2 0*3 10*2 0 10*2 0 10 (110 <D2:0) 0 10*2 0*2 10 0*2
		  10*3 0 (110 <D2:2) 10 0*2 10*2 0*2 10*3 0 10 0*2 10 0 10*2 0*3 10 0
		  10*3 0 (110 <D2:2) 10 0*2 10 0 (110 <D2:0) 10*2 0 10 0 10*2 0 10*2 0
		  10 0 10 0*2 10*2 0*3 10 0 10*3 0*3 10*2 0 10 0 (110 <D2:3) 10 0
		  (110 <D2:2) 10 (110 <D2:3) (110 <D2:3) (110 <D2:3) 10*2
		< 0*7 0 1*7

		# FinalStream, "consectetur adipiscing elit."
		< (1 10) (00111 00000 0010) (011 000 011 001 000 010) 0
		> 10 0*3 10 (110 <D2:1) 0*3 10*2 0 10 (110 <D2:0) 0 10*2 0*2 10*3 0 10*2
		  0 10*2 0*2 10*3 0 10 0 10 0*2 10*2 0 10*2 0*3 10*2 0*3 10 0 10*3 0 10
		  0 10 0*2 10*2 0*3 10 0 10*3 0 10 0 10 0 10*3 0*2 10 0*2 10*3 0
		  (110 <D2:2) 10 0*2 10 0 (110 <D2:0) 10*2 0*3 10 0*2 10*2 0 10 0*2 10 0
		  10*2 0 (110 <D2:1) 10*3 0 10 0*2 10 0 10*2 0 10*2 0*2 10*3 0 10*2 0*3
		  10*2 0 10 0*2 10 0 10*2 0*2 10*3 0 10*2 0 10*3 0*2 10*2 0 (110 <D2:2)
		  10 0*2 10 0 10 0*2 10*2 0*3 10*2 0 10*2 0 10 0*2 10 0 10*2 0*3 10 0
		  10*3 0*2 10*3 0 10 (111 <D7:3) 10 (110 <D2:3) (110 <D2:3)
		< 0*7 0 1*7

		# FinalNil, "Do not communicate by sharing"
		< (0 10) (00101 00000 0010) (011 000 011 001 000 010) 0
		> 0*2 10 0 10*3 0*2 10 0*3 10 0 10 (110 <D2:0) 0 10*2 0 (110 <D2:2) 10
		  0*3 10*3 0 10*2 0 10 (110 <D2:0) 0 10*2 0*3 10 0 10*3 0 (110 <D2:2)
		  10 0*2 10*2 0*3 10*2 0 10 (110 <D2:0) 0 10*2 0 10 0 10*2 0 10*2 0 10 0
		  10*2 0 10*2 0 10 0 10 0 10*3 0*2 10*3 0 10*2 0 10 0*2 10 0 10*2 0 10*2
		  0*3 10*2 0 10 0 (110 <D2:0) 10*2 0*3 10 0 10*3 0 10 0 10 0*2 10*2 0
		  (110 <D2:2) 10 0*3 10 0*3 10*2 0 10 0*2 10 (110 <D2:0) 0 (110 <D2:2)
		  10 0*2 10*2 0*2 10*3 0 (110 <D2:0) 10 0 10*2 0 10 0 (110 <D2:0) 10*2
		  0*2 10 0*2 10*3 0 10 0*2 10 0 10*2 0*2 10*3 0 10*2 0 10*3 0*2 10*2 0
		  (110 <D2:3) 10 (110 <D2:3) (110 <D2:1)
		< 0*5 0 1*7

		# FinalNil, " memory; instead, share memor"
		< (0 10) (00110 00000 0010) (011 000 011 001 000 010) 0
		> 0*2 10 0 10*3 0 (110 <D2:1) 10 0*2 10 0 10*2 0 10*2 0 10 0 10 0*2 10*2
		  0 10 0 10*2 0 10*2 0 10 (110 <D2:0) 0 10*2 0*2 10 0*2 10*3 0 10 0*2 10
		  (110 <D2:0) 0 10*2 0 10*3 0 (110 <D2:3) 10 0*2 10 0*2 10 0 10*2 0*2
		  10*3 0 10*2 0 10*2 0*2 10*3 0*3 10 0 10*3 0 10 0 10 0*2 10*2 0 10 0
		  (110 <D2:0) 10*2 0*3 10 0*2 10*2 0*3 10*2 0 10 0 (110 <D2:3) 10 0*2
		  10*2 0*2 10*3 0 (110 <D2:0) 10 0 10*2 0 10 0 (110 <D2:0) 10*2 0*2 10
		  0*2 10*3 0 10 0 10 0*2 10*2 0 (110 <D2:2) 10 0*2 10 0 10*2 0 10*2 0 10
		  0 10 0*2 10*2 0 10 0 10*2 0 10*2 0 10 (110 <D2:0) 0 10*2 0*2 10 0*2
		  10*3 0 (110 <D2:2) 10 (110 <D2:3) (110 <D2:2)
		< 0*6 0 1*7

		# FinalNil, "y by communicating."
		< (0 10) (00110 00000 0010) (011 000 011 001 000 010) 0
		> 0 10*3 0*2 10 0 10*2 0 (110 <D2:0) 10 (110 <D2:2) 0 10*3 0 10*3 0*2 10
		  0 10*2 0 (110 <D2:0) 10 (110 <D2:2) 0 10*2 0*2 10*3 0*2 10 0
		  (110 <D2:0) 10 0*2 10 0 10 0*2 10 0*2 10 0 10 0*2 10 0*2 10 0 10 0 10
		  0*3 10*2 0*3 10 0*2 10 0 10*2 0 10 0*2 10 0*2 10*3 0*2 10 0 10
		  (110 <D2:0) 0*2 10*3 0 10 0*3 10 0 10*2 0 10 0*2 10*2 0*3 10 0*2 10
		  0*3 10*2 0*2 10*2 0*3 10 0 10*2 (111 <D7:36) 10 (110 <D2:3)
		  (110 <D2:3) (110 <D2:3) (110 <D2:3) (110 <D2:3) (110 <D2:3)
		  (110 <D2:3) (110 <D2:3) 10
		< 0*6 0 1*7
	`)
	vectors := []struct {
		data                   string
		inOff, outOff, numBlks int64
		final                  FinalMode
	}{{
		"The quick brown fox jumped over the lazy dog.",
		93, 45, 2, FinalMeta,
	}, {
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit.",
		104, 56, 2, FinalStream,
	}, {
		"Do not communicate by sharing memory; instead, share memory by communicating.",
		148, 77, 3, FinalNil,
	}}

	rd := bytes.NewReader(data)
	for i, v := range vectors {
		mr.Reset(rd)
		buf, err := ioutil.ReadAll(mr)
		if err != nil {
			t.Errorf("test %d, unexpected error: ReadAll() = %v", i, err)
		}
		if str := string(buf); str != v.data {
			t.Errorf("test %d, output mismatch:\ngot  %s\nwant %s", i, str, v.data)
		}
		if err := mr.Close(); err != nil {
			t.Errorf("test %d, unexpected error: Close() = %v", i, err)
		}
		if mr.InputOffset != v.inOff {
			t.Errorf("test %d, input offset mismatch, got %d, want %d", i, mr.InputOffset, v.inOff)
		}
		if mr.OutputOffset != v.outOff {
			t.Errorf("test %d, output offset mismatch, got %d, want %d", i, mr.OutputOffset, v.outOff)
		}
		if mr.NumBlocks != v.numBlks {
			t.Errorf("test %d, block count mismatch, got %d, want %d", i, mr.NumBlocks, v.numBlks)
		}
		if mr.FinalMode != v.final {
			t.Errorf("test %d, final mode mismatch, got %d, want %d", i, mr.FinalMode, v.final)
		}
	}
}

func BenchmarkReader(b *testing.B) {
	data := make([]byte, 1<<16)
	rand.Read(data)

	rd := new(bytes.Reader)
	bb := new(bytes.Buffer)
	mr := new(Reader)

	mw := NewWriter(bb)
	mw.Write(data)
	mw.Close()

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rd.Reset(bb.Bytes())
		mr.Reset(rd)

		cnt, err := io.Copy(ioutil.Discard, mr)
		if cnt != int64(len(data)) || err != nil {
			b.Fatalf("Copy() = (%d, %v), want (%d, nil)", cnt, err, len(data))
		}
		if err := mr.Close(); err != nil {
			b.Fatalf("Close() = %v, want nil", err)
		}
	}
}
