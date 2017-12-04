// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package meta

import (
	"bytes"
	"io"
	"math/rand"
	"testing"

	"github.com/dsnet/compress/internal/errors"
	"github.com/dsnet/compress/internal/testutil"
)

// TestWriter tests that the encoded output matches the expected output exactly.
// A failure here does not necessarily mean that the encoder is wrong since
// there are many possible representations. Before changing the test vectors to
// make a test pass, one must verify the new output is correct.
func TestWriter(t *testing.T) {
	db := testutil.MustDecodeBitGen
	dh := testutil.MustDecodeHex

	errFuncs := map[string]func(error) bool{
		"IsInvalid": errors.IsInvalid,
	}
	vectors := []struct {
		desc   string    // Description of the text
		input  []byte    // Test input string
		output []byte    // Expected output string
		final  FinalMode // Input final mode
		errf   string    // Name of error checking callback
	}{{
		desc:  "empty meta block (FinalNil)",
		input: dh(""),
		output: db(`<<<
			< (0 10) (00011 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> (111 <D7:127) (111 <D7:99) 10 (110 <D2:3) 10
			< 0*3 0 1*3
		`),
		final: FinalNil,
	}, {
		desc:  "empty meta block (FinalMeta)",
		input: dh(""),
		output: db(`<<<
			< (0 10) (00011 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> 10 (111 <D7:127) (111 <D7:99) 10 (110 <D2:3)
			< 0*3 0 1*3
		`),
		final: FinalMeta,
	}, {
		desc:  "input string 'a'",
		input: []byte("a"),
		output: db(`<<<
			< (0 10) (00110 00000 1000) (011 000 011 001 000 (000 000)*3 010) 0
			> 10 0 10 0 (110 <D2:0) 10 0 (110 <D2:0) 10*2 (111 <D7:127)
			  (111 <D7:82) 10 (110 <D2:3) (110 <D2:1)
			< 0*6 0 1*4
		`),
		final: FinalMeta,
	}, {
		desc:  "input string 'ab'",
		input: []byte("ab"),
		output: db(`<<<
			< (0 10) (00000 00000 1000) (011 000 011 001 000 (000 000)*3 010) 0
			> 10 0*2 10 0*3 10 0 (110 <D2:0) 10*2 0*2 10 0*3 10*2 (111 <D7:127)
			  (111 <D7:77) 10 (110 <D2:3) 10
			< 0*0 0 1*4
		`),
		final: FinalMeta,
	}, {
		desc:  "input string 'abc'",
		input: []byte("abc"),
		output: db(`<<<
			< (0 10) (00000 00000 0110) (011 000 011 001 000 (000 000)*2 010) 0
			> 10 0 10*2 0*3 10 0 (110 <D2:0) 10*2 0*2 10 0*3 10*2 0 10*2 0*3
			  10*2 (111 <D7:127) (111 <D7:58) 10 (110 <D2:3)*2 (110 <D2:3)
			< 0*0 0 1*5
		`),
		final: FinalMeta,
	}, {
		desc:  "input string 'Hello, world!' with FinalNil",
		input: dh("48656c6c6f2c20776f726c6421"),
		output: db(`<<<
			< (0 10) (00111 00000 0100) (011 000 011 001 000 (000 000)*1 010) 0
			> 0*2 10 0 10*2 0 (110 <D2:0) 10 0*2 10 0 10 0 10 0*2 10*2 0*3 10*2
			  0 10*2 0*3 10*2 0 10*2 0 10 (110 <D2:0) 0 10*2 0*3 10*2 0 10 0
			  (110 <D2:3) 10 0*2 10*3 0 10*3 0 10 (110 <D2:0) 0 10*2 0*2 10 0*2
			  10*3 0*3 10*2 0 10*2 0*3 10 0*2 10*2 0 10 0 (110 <D2:0) 10
			  (111 <D7:124) 10 (110 <D2:3) (110 <D2:2)
			< 0*7 0 1*6
		`),
		final: FinalNil,
	}, {
		desc:  "input string 'Hello, world!' with FinalMeta",
		input: dh("48656c6c6f2c20776f726c6421"),
		output: db(`<<<
			< (0 10) (00110 00000 0100) (011 000 011 001 000 (000 000)*1 010) 0
			> 10 0 10 0 10*2 0 (110 <D2:0) 10 0*2 10 0 10 0 10 0*2 10*2 0*3 10*2
			  0 10*2 0*3 10*2 0 10*2 0 10 (110 <D2:0) 0 10*2 0*3 10*2 0 10 0
			  (110 <D2:3) 10 0*2 10*3 0 10*3 0 10 (110 <D2:0) 0 10*2 0*2 10 0*2
			  10*3 0*3 10*2 0 10*2 0*3 10 0*2 10*2 0 10 0 (110 <D2:0) 10
			  (111 <D7:125) 10 (110 <D2:3) (110 <D2:1)
			< 0*6 0 1*6
		`),
		final: FinalMeta,
	}, {
		desc:  "input string 'Hello, world!' with FinalStream",
		input: dh("48656c6c6f2c20776f726c6421"),
		output: db(`<<<
			< (1 10) (00110 00000 0100) (011 000 011 001 000 (000 000)*1 010) 0
			> 10 0 10 0 10*2 0 (110 <D2:0) 10 0*2 10 0 10 0 10 0*2 10*2 0*3 10*2
			  0 10*2 0*3 10*2 0 10*2 0 10 (110 <D2:0) 0 10*2 0*3 10*2 0 10 0
			  (110 <D2:3) 10 0*2 10*3 0 10*3 0 10 (110 <D2:0) 0 10*2 0*2 10 0*2
			  10*3 0*3 10*2 0 10*2 0*3 10 0*2 10*2 0 10 0 (110 <D2:0) 10
			  (111 <D7:125) 10 (110 <D2:3) (110 <D2:1)
			< 0*6 0 1*6
		`),
		final: FinalStream,
	}, {
		desc:  "input hex-string '00'*4",
		input: dh("00000000"),
		output: db(`<<<
			< (0 10) (00110 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> 10 0*3 10 (111 <D7:127) (111 <D7:96) 10 (110 <D2:2)
			< 0*6 0 1*3
		`),
		final: FinalMeta,
	}, {
		desc:  "input hex-string '00'*8",
		input: dh("0000000000000000"),
		output: db(`<<<
			< (0 10) (00011 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> 10 0 (110 <D2:0) 10 (111 <D7:127) (111 <D7:95) 10 (110 <D2:2)
			< 0*3 0 1*3
		`),
		final: FinalMeta,
	}, {
		desc:  "input hex-string '00'*16",
		input: dh("00000000000000000000000000000000"),
		output: db(`<<<
			< (0 10) (00011 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> 10 0 (110 <D2:1) 10 (111 <D7:127) (111 <D7:94) 10 (110 <D2:2)
			< 0*3 0 1*3
		`),
		final: FinalMeta,
	}, {
		desc:  "input hex-string 'ff'*4",
		input: dh("ffffffff"),
		output: db(`<<<
			< (0 10) (00101 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> 10*2 0*2 10 (111 <D7:127) (111 <D7:97) 10 (110 <D2:1)
			< 0*5 0 1*3
		`),
		final: FinalMeta,
	}, {
		desc:  "input hex-string 'ff'*8",
		input: dh("ffffffffffffffff"),
		output: db(`<<<
			< (0 10) (00100 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> 10*2 0*3 10 (111 <D7:127) (111 <D7:96) 10 (110 <D2:1)
			< 0*4 0 1*3
		`),
		final: FinalMeta,
	}, {
		desc:  "input hex-string 'ff'*16",
		input: dh("ffffffffffffffffffffffffffffffff"),
		output: db(`<<<
			< (0 10) (00001 00000 1010) (011 000 011 001 000 (000 000)*4 010) 0
			> 10*2 0 (110 <D2:0) 10 (111 <D7:127) (111 <D7:95) 10 (110 <D2:1)
			< 0*1 0 1*3
		`),
		final: FinalMeta,
	}, {
		desc:  "the random hex-string '911fe47084a4668b'",
		input: dh("911fe47084a4668b"),
		output: db(`<<<
			< (0 10) (00100 00000 0100) (011 000 011 001 000 (000 000)*1 010) 0
			> 10 0 (110 <D2:0) 10 0 10 0*3 10 0*2 10 (110 <D2:2) 0 (110 <D2:1)
			  10 0*2 10*3 0 (110 <D2:0) 10*3 0*3 10 0 (110 <D2:0) 10 0*2 10 0*2
			  10 0 10 0 10*2 0*2 10*2 0 10*2 0 10 0*3 10 (111 <D7:127)
			  (111 <D7:2) 10 (110 <D2:3)*5 (110 <D2:0)
			< 0*4 0 1*6
		`),
		final: FinalMeta,
	}, {
		desc:  "the random hex-string 'de9fa94cb16f40fc'",
		input: dh("de9fa94cb16f40fc"),
		output: db(`<<<
			< (0 10) (00001 00000 0100) (011 000 011 001 000 (000 000)*1 010) 0
			> 10*2 0*3 10 0 10 0 (110 <D2:0) 10 0 (110 <D2:3) 10*2 0*2 10*2 0 10
			  0 10 0 10*2 0*2 10*2 0 10 0 10*3 0*2 10 0 (110 <D2:1) 10 0*2 10
			  (110 <D2:3) 0 10*3 (111 <D7:127) (111 <D7:9) 10 (110 <D2:3)*5 10*2
			< 0*1 0 1*6
		`),
		final: FinalMeta,
	}, {
		desc:  "input hex-string '55'*22",
		input: dh("55555555555555555555555555555555555555555555"),
		output: db(`<<<
			< (1 10) (00000 00000 0010) (011 000 011 001 000 (000 000)*0 010) 0
			> 10 0*2 10*2 0 10*2 (0 10)*87 (111 <D7:27) 10 (110 <D2:3)*5 (110 <D2:2)
			< 0*0 0 1*7
		`),
		final: FinalStream,
	}, {
		desc:  "input hex-string '55'*23",
		input: dh("5555555555555555555555555555555555555555555555"),
		output: db(`<<<
			< (0 10) (00000 00000 0010) (011 000 011 001 000 (000 000)*0 010) 0
			> 10 0 10*3 0 10*2 (0 10)*91 (111 <D7:24) 10 (110 <D2:3)*5
			< 0*0 0 1*7
		`),
		final: FinalMeta,
	}, {
		desc:  "input hex-string '55'*24",
		input: dh("555555555555555555555555555555555555555555555555"),
		output: db(`<<<
			< (0 10) (00111 00000 0010) (011 000 011 001 000 010)
			> 0 (110 <D2:2) 10*3 (0 10)*95 (111 <D7:17) 10 (110 <D2:3)*4 (110 <D2:2)
			< 0*7 0 1*7
		`),
		final: FinalNil,
	}, {
		desc:  "input hex-string '55'*25",
		input: dh("55555555555555555555555555555555555555555555555555"),
		output: db(`<<<
			< (1 10) (00110 00000 0010) (011 000 011 001 000 010)
			> 0 10 0 10 0*2 10*3 (0 10)*99 (111 <D7:15) 10 (110 <D2:3)*3 (110 <D2:2)
			< 0*6 0 1*7
		`),
		final: FinalStream,
	}, {
		desc:  "input hex-string '55'*26",
		input: dh("5555555555555555555555555555555555555555555555555555"),
		output: db(`<<<
			< (0 10) (00101 00000 0010) (011 000 011 001 000 010)
			> 0 10 0*2 10 0 10*3 (0 10)*103 (111 <D7:11) 10 (110 <D2:3)*3 10
			< 0*5 0 1*7
		`),
		final: FinalMeta,
	}, {
		desc:  "input hex-string '55'*27",
		input: dh("555555555555555555555555555555555555555555555555555555"),
		output: db(`<<<
			< (0 10) (00011 00000 0010) (011 000 011 001 000 010)
			> 0*3 10*2 0 10*3 (0 10)*107 (111 <D7:7) 10 (110 <D2:3)*2 (110 <D2:0)
			< 0*3 0 1*7
		`),
		final: FinalNil,
	}, {
		desc:  "input hex-string '55'*28",
		input: dh("55555555555555555555555555555555555555555555555555555555"),
		output: db(`<<<
			< (1 10) (00101 00000 0010) (011 000 011 001 000 010)
			> 0 10 0*3 10 (110 <D2:0) (0 10)*111 (111 <D7:3) 10 (110 <D2:3) (110 <D2:2)
			< 0*5 0 1*7
		`),
		final: FinalStream,
	}, {
		desc:  "input hex-string '55'*29",
		input: dh("5555555555555555555555555555555555555555555555555555555555"),
		output: db(`<<<
			< (0 10) (00101 00000 0010) (011 000 011 001 000 010)
			> 0 10 0 10 0 10 (110 <D2:0) (0 10)*115 (111 <D7:0) 10 (110 <D2:3)
			< 0*5 0 1*7
		`),
		final: FinalMeta,
	}, {
		desc:  "input hex-string '55'*30",
		input: dh("555555555555555555555555555555555555555555555555555555555555"),
		output: db(`<<<
			< (0 10) (00110 00000 0010) (011 000 011 001 000 010)
			> 0 (110 <D2:0) 10 (110 <D2:1) (0 10)*119 0 (110 <D2:2) 10 (110 <D2:0)
			< 0*6 0 1*7
		`),
		final: FinalNil,
	}, {
		desc:  "input hex-string '55'*31",
		input: dh("55555555555555555555555555555555555555555555555555555555555555"),
		final: FinalStream,
		errf:  "IsInvalid",
	}, {
		desc:  "input hex-string '55'*32",
		input: dh("5555555555555555555555555555555555555555555555555555555555555555"),
		final: FinalMeta,
		errf:  "IsInvalid",
	}, {
		desc:  "input hex-string '73de76bebcf69d5fed3fb3cee87bacfd7de876facffedf'",
		input: dh("73de76bebcf69d5fed3fb3cee87bacfd7de876facffedf"),
		output: db(`<<<
			< (0 10) (00010 00000 0100) (011 000 011 001 000 (000 000) 010)
			> 0*2 10 (110 <D2:0) 0 10 0*2 10*2 0*3 10*2 0 (110 <D2:0) 10 0*2 10
			  0*2 10 0*3 10*2 0 (110 <D2:1) 10 0 10*2 0 (110 <D2:0) 10 0 10 0*2
			  10 0 (110 <D2:1) 10 0*3 10*2 0 (110 <D2:2) 10 0 10 0 10 0*2 10 0
			  (110 <D2:3) 0*2 10*2 0*2 10*2 0*2 10 0 10 0*3 10*2 0*2 10*3 0 10 0
			  (110 <D2:1) 10 0 (110 <D2:0) 10*3 0*2 10 0 10 0*2 10 0 (110 <D2:3)
			  10 0 (110 <D2:1) 10 (110 <D2:0) 0 10 0*3 10 0*2 10 0*3 10*2 0 10 0
			  (110 <D2:3) 0*2 10*2 0*2 10 (111 <D7:1) 10 (111 <D7:53) 10*3
			< 0*2 0 1*6
		`),
		final: FinalNil,
	}, {
		desc:  "input hex-string '73de76bebcf69d5fed3fb3cee87bacfd7de876facffede'",
		input: dh("73de76bebcf69d5fed3fb3cee87bacfd7de876facffede"),
		final: FinalStream,
		errf:  "IsInvalid",
	}, {
		desc:  "input hex-string 'def773bfab15d257ffffffbbafdf3fef6e1fefd6e75ffffff6fefcff67d9'",
		input: dh("def773bfab15d257ffffffbbafdf3fef6e1fefd6e75ffffff6fefcff67d9"),
		output: db(`<<<
			< (0 10) (00111 00000 0100) (011 000 011 001 000 (000 000) 010)
			> 0 10*2 0 10 (110 <D2:1) 0 (110 <D2:0) 10 0 (110 <D2:1) 10 0
			  (110 <D2:2) 10*2 0*3 10 0 (110 <D2:2) 10 0*3 10 0 10 0 10 0*2 10 0
			  10 0 10 (110 <D2:0) 0 10*2 0 10 0 (110 <D2:1) 10 0 10 0 10
			  (111 <D7:15) 10 0*3 10 0 (110 <D2:1) 10 0 10 0 (110 <D2:2) 10 0
			  (110 <D2:3) 0 10*2 0 (110 <D2:0) 10 0*3 10 0*3 10 0*2 10 0
			  (110 <D2:1) 10*3 0 (110 <D2:0) 10 0*3 10 0*2 10 0 10 0 (110 <D2:1)
			  10*2 0 (110 <D2:3) 0 10 0 10 (111 <D7:5) 10 0*2 10 0 (110 <D2:0)
			  10 0 (110 <D2:3) 10*2 (111 <D7:6) 10*2 0*2 10 0 10*2 0*2 10 0
			  (110 <D2:3) 0 10*3
			< 0*7 0 1*6
		`),
		final: FinalMeta,
	}, {
		desc:  "input hex-string 'dff773bfab15d257ffffffbbafdf3fef6e1fefd6e75ffffff6fefcff67d9'",
		input: dh("dff773bfab15d257ffffffbbafdf3fef6e1fefd6e75ffffff6fefcff67d9"),
		final: FinalMeta,
		errf:  "IsInvalid",
	}}

	for i, v := range vectors {
		var b bytes.Buffer
		mw := NewWriter(&b)
		mw.bufCnt = copy(mw.buf[:], v.input)
		for _, b := range v.input {
			b0s, b1s := numBits(b)
			mw.buf0s, mw.buf1s = b0s+mw.buf0s, b1s+mw.buf1s
		}
		err := mw.encodeBlock(v.final)
		output := b.Bytes()

		if got, want, ok := testutil.BytesCompare(output, v.output); !ok {
			t.Errorf("test %d (%s), mismatching data:\ngot  %s\nwant %s", i, v.desc, got, want)
		}
		if len(output) != int(mw.OutputOffset) {
			t.Errorf("test %d (%s), mismatching offset: got %d, want %d", i, v.desc, len(output), mw.OutputOffset)
		}
		if v.errf != "" && !errFuncs[v.errf](err) {
			t.Errorf("test %d (%s), mismatching error:\ngot %v\nwant %s(got) == true", i, v.desc, err, v.errf)
		} else if v.errf == "" && err != nil {
			t.Errorf("test %d (%s), unexpected error: got %v", i, v.desc, err)
		}
	}
}

type faultyWriter struct{}

func (faultyWriter) Write([]byte) (int, error) { return 0, io.ErrShortWrite }

func TestWriterReset(t *testing.T) {
	bb := new(bytes.Buffer)
	mw := NewWriter(bb)
	buf := make([]byte, 512)

	// Test Writer for idempotent Close.
	if err := mw.Close(); err != nil {
		t.Errorf("unexpected error, Close() = %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Errorf("unexpected error, Close() = %v", err)
	}
	if _, err := mw.Write(buf); err != errClosed {
		t.Errorf("unexpected error, Write(...) = %v, want %v", err, errClosed)
	}

	// Test Writer with faulty writer.
	mw.Reset(faultyWriter{})
	if _, err := mw.Write(buf); err != io.ErrShortWrite {
		t.Errorf("unexpected error, Write(...) = %v, want %v", err, io.ErrShortWrite)
	}
	if err := mw.Close(); err != io.ErrShortWrite {
		t.Errorf("unexpected error, Close() = %v, want %v", err, io.ErrShortWrite)
	}

	// Test Writer in normal use.
	bb.Reset()
	mw.Reset(bb)
	data := []byte("The quick brown fox jumped over the lazy dog.")
	cnt, err := mw.Write(data)
	if err != nil {
		t.Errorf("unexpected error, Write(...) = %v", err)
	}
	if cnt != len(data) {
		t.Errorf("write count mismatch, got %d, want %d", cnt, len(data))
	}
	if err := mw.Close(); err != nil {
		t.Errorf("unexpected error, Close() = %v", err)
	}
	if mw.InputOffset != int64(len(data)) {
		t.Errorf("input offset mismatch, got %d, want %d", mw.InputOffset, len(data))
	}
	if mw.OutputOffset != int64(bb.Len()) {
		t.Errorf("output offset mismatch, got %d, want %d", mw.OutputOffset, bb.Len())
	}
}

func BenchmarkWriter(b *testing.B) {
	data := make([]byte, 1<<16)
	rand.Read(data)

	rd := new(bytes.Reader)
	bb := new(bytes.Buffer)
	mw := new(Writer)

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rd.Reset(data)
		bb.Reset()
		mw.Reset(bb)

		cnt, err := io.Copy(mw, rd)
		if cnt != int64(len(data)) || err != nil {
			b.Fatalf("Copy() = (%d, %v), want (%d, nil)", cnt, err, len(data))
		}
		if err := mw.Close(); err != nil {
			b.Fatalf("Close() = %v, want nil", err)
		}
	}
}
