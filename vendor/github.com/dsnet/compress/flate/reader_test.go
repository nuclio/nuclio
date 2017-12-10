// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package flate

import (
	"bytes"
	"compress/flate"
	"flag"
	"io"
	"io/ioutil"
	"os/exec"
	"strings"
	"testing"

	"github.com/dsnet/compress/internal/errors"
	"github.com/dsnet/compress/internal/testutil"
)

var zcheck = flag.Bool("zcheck", false, "verify reader test vectors with C zlib library")

// pyDecompress decompresses the input by using the Python wrapper library
// over the C zlib library:
//
//	>>> hex_string = "010100feff11"
//	>>> import zlib
//	>>> zlib.decompress(hex_string.decode("hex"), -15) # Negative means raw DEFLATE
//	'\x11'
//
func pyDecompress(input []byte) ([]byte, error) {
	var buf bytes.Buffer
	cmd := exec.Command("python", "-c", "import sys, zlib; sys.stdout.write(zlib.decompress(sys.stdin.read(), -15))")
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = &buf
	err := cmd.Run()
	return buf.Bytes(), err
}

func TestReader(t *testing.T) {
	db := testutil.MustDecodeBitGen
	dh := testutil.MustDecodeHex

	errFuncs := map[string]func(error) bool{
		"IsUnexpectedEOF": func(err error) bool { return err == io.ErrUnexpectedEOF },
		"IsCorrupted":     errors.IsCorrupted,
	}
	vectors := []struct {
		name   string // Sub-test name
		input  []byte // Test input string
		output []byte // Expected output string
		inIdx  int64  // Expected input offset after reading
		outIdx int64  // Expected output offset after reading
		errf   string // Name of error checking callback
	}{{
		name: "EmptyString",
		errf: "IsUnexpectedEOF",
	}, {
		name: "RawBlock",
		input: db(`<<<
			< 0 00 0*5          # Non-last, raw block, padding
			< H16:000c H16:fff3 # RawSize: 12
			"hello, world"      # Raw data

			< 1 10    # Last, fixed block
			> 0000000 # EOB marker
		`),
		output: []byte("hello, world"),
		inIdx:  19,
		outIdx: 12,
		errf:   "IsUnexpectedEOF",
	}, {
		name: "RawBlockNonZeroPadding",
		input: db(`<<<
			< 1 00 10101        # Last, raw block, non-zero padding
			< H16:0001 H16:fffe # RawSize: 1
			X:11                # Raw data
		`),
		output: dh("11"),
		inIdx:  6,
		outIdx: 1,
	}, {
		name: "RawBlockShortest",
		input: db(`<<<
			< 1 00 0*5          # Last, raw block, padding
			< H16:0000 H16:ffff # RawSize: 0
		`),
		inIdx: 5,
	}, {
		name: "RawBlockLongest",
		input: db(`<<<
			< 1 00 0*5          # Last, raw block, padding
			< H16:ffff H16:0000 # RawSize: 65535
			X:7a*65535
		`),
		output: db("<<< X:7a*65535"),
		inIdx:  65540,
		outIdx: 65535,
	}, {
		name: "RawBlockBadSize",
		input: db(`<<<
			< 1 00 0*5          # Last, raw block, padding
			< H16:0001 H16:fffd # RawSize: 1
			X:11                # Raw data
		`),
		inIdx: 5,
		errf:  "IsCorrupted",
	}, {
		// Truncated after block header.
		name: "RawBlockTruncated0",
		input: db(`<<<
			< 0 00 0*5 # Non-last, raw block, padding
		`),
		inIdx: 1,
		errf:  "IsUnexpectedEOF",
	}, {
		// Truncated inside size field.
		name: "RawBlockTruncated1",
		input: db(`<<<
			< 0 00 0*5 # Non-last, raw block, padding
			< H8:0c    # RawSize: 12
		`),
		inIdx: 1,
		errf:  "IsUnexpectedEOF",
	}, {
		// Truncated after size field.
		name: "RawBlockTruncated2",
		input: db(`<<<
			< 0 00 0*5 # Non-last, raw block, padding
			< H16:000c # RawSize: 12
		`),
		inIdx: 3,
		errf:  "IsUnexpectedEOF",
	}, {
		// Truncated before raw data.
		name: "RawBlockTruncated3",
		input: db(`<<<
			< 0 00 0*5          # Non-last, raw block, padding
			< H16:000c H16:fff3 # RawSize: 12
		`),
		inIdx: 5,
		errf:  "IsUnexpectedEOF",
	}, {
		// Truncated inside raw data.
		name: "RawBlockTruncated4",
		input: db(`<<<
			< 0 00 0*5          # Non-last, raw block, padding
			< H16:000c H16:fff3 # RawSize: 12
			"hello"             # Raw data
		`),
		output: []byte("hello"),
		inIdx:  10,
		outIdx: 5,
		errf:   "IsUnexpectedEOF",
	}, {
		// Truncated before next block.
		name: "RawBlockTruncated5",
		input: db(`<<<
			< 0 00 0*5          # Non-last, raw block, padding
			< H16:000c H16:fff3 # RawSize: 12
			"hello, world"      # Raw data
		`),
		output: []byte("hello, world"),
		inIdx:  17,
		outIdx: 12,
		errf:   "IsUnexpectedEOF",
	}, {
		// Truncated after fixed block header.
		name: "FixedBlockTruncated0",
		input: db(`<<<
			< 0 01 # Non-last, fixed block
		`),
		inIdx:  1,
		outIdx: 0,
		errf:   "IsUnexpectedEOF",
	}, {
		// Truncated after mid-block and mid-symbol.
		name: "FixedBlockTruncated1",
		input: db(`<<<
			< 0 01 # Non-last, fixed block
			> 01111000 10010101 10011 # Truncate 100 from last symbol
		`),
		output: []byte("He"),
		inIdx:  3,
		outIdx: 2,
		errf:   "IsUnexpectedEOF",
	}, {
		// Truncated after mid-block and post-symbol.
		name: "FixedBlockTruncated2",
		input: db(`<<<
			< 0 01 # Non-last, fixed block
			> 01111000 10010101 10011100 110010000*5
		`),
		output: []byte("Hel\x90\x90\x90\x90\x90"),
		inIdx:  9,
		outIdx: 8,
		errf:   "IsUnexpectedEOF",
	}, {
		// Truncated after mid-block and post-EOB.
		name: "FixedBlockTruncated3",
		input: db(`<<<
			< 0 01 # Non-last, fixed block
			> 01111000 10010101 10011100 110010000*5
			> 0000000 # EOB marker
		`),
		output: []byte("Hel\x90\x90\x90\x90\x90"),
		inIdx:  10,
		outIdx: 8,
		errf:   "IsUnexpectedEOF",
	}, {
		name: "FixedBlockShortest",
		input: db(`<<<
			< 1 01    # Last, fixed block
			> 0000000 # EOB marker
		`),
		inIdx: 2,
	}, {
		name: "FixedBlockHelloWorld",
		input: db(`<<<
			< 1 01    # Last, fixed block

			> 01111000 10010101 10011100 10011100 10011111 01011100 01010000
			  10100111 10011111 10100010 10011100 10010100 01010001
			> 0000000 # EOB marker
		`),
		output: []byte("Hello, world!"),
		inIdx:  15,
		outIdx: 13,
	}, {
		// Make sure the use of a dynamic block, following a fixed block does
		// not alter the global Decoder tables.
		name: "FixedDynamicFixedDynamicBlocks",
		input: db(`<<<
			< 0 01               # Non-last, fixed block
			> 00110000 0000000   # Compressed data

			< 0 10               # Non-last, dynamic block
			< D5:0 D5:3 D4:15    # HLit: 257, HDist: 4, HCLen: 19
			< 000*3 001*2 000*14 # HCLens: {0:1, 8:1}
			> 0 1*256 0*4        # HLits: {*:8}, HDists: {}
			> 00000000 11111111  # Compressed data

			< 0 01               # Non-last, fixed block
			> 00110000 0000000   # Compressed data

			< 1 10               # Last, dynamic block
			< D5:0 D5:3 D4:15    # HLit: 257, HDist: 4, HCLen: 19
			< 000*3 001*2 000*14 # HCLens: {0:1, 8:1}
			> 0 1*256 0*4        # HLits: {*:8}, HDists: {}
			> 00000000 11111111  # Compressed data
		`),
		output: dh("00010001"),
		inIdx:  93,
		outIdx: 4,
	}, {
		name: "ReservedBlock",
		input: db(`<<<
			< 1 11 0*5 # Last, reserved block, padding
			X:deadcafe # ???
		`),
		inIdx: 1,
		errf:  "IsCorrupted",
	}, {
		// Use reserved HLit symbol 287 in fixed block.
		name: "ReservedHLitSymbol",
		input: db(`<<<
			< 1 01              # Last, fixed block
			> 01100000 11000111 # Use invalid symbol 287
		`),
		output: dh("30"),
		inIdx:  3,
		outIdx: 1,
		errf:   "IsCorrupted",
	}, {
		// Use reserved HDist symbol 30 in fixed block.
		name: "ReservedHDistSymbol",
		input: db(`<<<
			< 1 01                   # Last, fixed block
			> 00110000 0000001 D5:30 # Use invalid HDist symbol 30
			> 0000000                # EOB marker
		`),
		output: dh("00"),
		inIdx:  3,
		outIdx: 1,
		errf:   "IsCorrupted",
	}, {
		// Degenerate HCLenTree.
		name: "HuffmanTree00",
		input: db(`<<<
			< 1 10            # Last, dynamic block
			< D5:0 D5:0 D4:15 # HLit: 257, HDist: 1, HCLen: 19
			< 000*17 001 000  # HCLens: {1:1}
			> 0*256 1         # Use invalid HCLen code 1
		`),
		inIdx: 42,
		errf:  "IsCorrupted",
	}, {
		// Degenerate HCLenTree, empty HLitTree, empty HDistTree.
		name: "HuffmanTree01",
		input: db(`<<<
			< 1 10             # Last, dynamic block
			< D5:0 D5:0 D4:15  # HLit: 257, HDist: 1, HCLen: 19
			< 000*3 001 000*15 # HCLens: {0:1}
			> 0*258            # HLits: {}, HDists: {}
		`),
		inIdx: 42,
		errf:  "IsCorrupted",
	}, {
		// Empty HCLenTree.
		name: "HuffmanTree02",
		input: db(`<<<
			< 1 10            # Last, dynamic block
			< D5:0 D5:0 D4:15 # HLit: 257, HDist: 1, HCLen: 19
			< 000*19          # HCLens: {}
			> 0*258           # Use invalid HCLen code 0
		`),
		inIdx: 10,
		errf:  "IsCorrupted",
	}, {
		// Complete HCLenTree, complete HLitTree, empty HDistTree,
		// use missing HDist symbol.
		name: "HuffmanTree03",
		input: db(`<<<
			< 0 00 0*5                 # Non-last, raw block, padding
			< H16:0001 H16:fffe        # RawSize: 1
			X:7a                       # Raw data

			< 1 10                     # Last, dynamic block
			< D5:1 D5:0 D4:15          # HLit: 258, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 0*256 1*2                # HLits: {256:1, 257:1}
			> 0                        # HDists: {}
			> 1 0                      # Use invalid HDist code 0
		`),
		output: dh("7a"),
		inIdx:  48,
		outIdx: 1,
		errf:   "IsCorrupted",
	}, {
		// Complete HCLenTree, degenerate HLitTree, empty HDistTree.
		name: "HuffmanTree04",
		input: db(`<<<
			< 1 10                     # Last, dynamic block
			< D5:0 D5:0 D4:15          # HLit: 257, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 1 0*257                  # HLits: {0:1}, HDists: {}
			> 0*31 1                   # Use invalid HLit code 1
		`),
		output: db("<<< X:00*31"),
		inIdx:  46,
		outIdx: 31,
		errf:   "IsCorrupted",
	}, {
		// Complete HCLenTree, degenerate HLitTree, degenerate HDistTree.
		name: "HuffmanTree05",
		input: db(`<<<
			< 1 10                     # Last, dynamic block
			< D5:0 D5:0 D4:15          # HLit: 257, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 1 0*256 1                # HLits: {0:1}, HDists: {0:1}
			> 0*31 1                   # Use invalid HLit code 1
		`),
		output: db("<<< X:00*31"),
		inIdx:  46,
		outIdx: 31,
		errf:   "IsCorrupted",
	}, {
		// Complete HCLenTree, degenerate HLitTree, degenerate HDistTree,
		// use missing HLit symbol.
		name: "HuffmanTree06",
		input: db(`<<<
			< 1 10                     # Last, dynamic block
			< D5:0 D5:0 D4:15          # HLit: 257, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 0*256 1*2                # HLits: {256:1}, HDists: {0:1}
			> 1                        # Use invalid HLit code 1
		`),
		inIdx: 42,
		errf:  "IsCorrupted",
	}, {
		// Complete HCLenTree, complete HLitTree, too large HDistTree.
		name: "HuffmanTree07",
		input: db(`<<<
			< 1 10              # Last, dynamic block
			< D5:29 D5:31 D4:15 # HLit: 286, HDist: 32, HCLen: 19
			<1000011 X:05000000002004 X:00*39 X:04 # ???
		`),
		inIdx: 3,
		errf:  "IsCorrupted",
	}, {
		// Complete HCLenTree, complete HLitTree, empty HDistTree,
		// excessive repeater symbol.
		name: "HuffmanTree08",
		input: db(`<<<
			< 1 10                           # Last, dynamic block
			< D5:29 D5:29 D4:15              # HLit: 286, HDist: 30, HCLen: 19
			< 011 000 011 001 000*13 010 000 # HCLens: {0:0, 1:2, 16:3, 18:3}
			> 10 0*255 10 111 <D7:49 1       # Excessive repeater symbol
		`),
		inIdx: 43,
		errf:  "IsCorrupted",
	}, {
		// Complete HCLenTree, complete HLitTree, empty HDistTree of length 30.
		name: "HuffmanTree09",
		input: db(`<<<
			< 1 10               # Last, dynamic block
			< D5:0 D5:29 D4:15   # HLit: 257, HDist: 30, HCLen: 19
			< 000*3 001*2 000*14 # HCLens: {0:1, 8:1}
			> 0 1*256 0*30       # HLits: {*:8}, HDists: {}
			> 11111111           # Compressed data (has only EOB)
		`),
		inIdx: 47,
	}, {
		// Complete HCLenTree, complete HLitTree, under-subscribed HDistTree.
		name: "HuffmanTree10",
		input: db(`<<<
			< 1 10               # Last, dynamic block
			< D5:0 D5:29 D4:15   # HLit: 257, HDist: 30, HCLen: 19
			< 000*3 001*2 000*14 # HCLens: {0:1, 8:1}
			> 0 1*256 0*28 1*2   # HLits: {*:8}, HDists: {28:8, 29:8}
		`),
		inIdx: 46,
		errf:  "IsCorrupted",
	}, {
		// HDistTree of excessive length 31.
		name: "HuffmanTree11",
		input: db(`<<<
			< 1 10             # Last, dynamic block
			< D5:0 D5:30 D4:15 # HLit: 257, HDist: 31, HCLen: 19
			<0*7 X:240000000000f8 X:ff*31 X:07000000fc03 # ???
		`),
		inIdx: 3,
		errf:  "IsCorrupted",
	}, {
		// Complete HCLenTree, over-subscribed HLitTree.
		name: "HuffmanTree12",
		input: db(`<<<
			< 1 10               # Last, dynamic block
			< D5:0 D5:0 D4:15    # HLit: 257, HDist: 1, HCLen: 19
			< 000*3 001*2 000*14 # HCLens: {0:1, 8:1}
			> 1*257 0            # HLits: {*:8}
			<0*4 X:f00f          # ???
		`),
		inIdx: 42,
		errf:  "IsCorrupted",
	}, {
		// Complete HCLenTree, under-subscribed HLitTree.
		name: "HuffmanTree13",
		input: db(`<<<
			< 1 10               # Last, dynamic block
			< D5:0 D5:0 D4:15    # HLit: 257, HDist: 1, HCLen: 19
			< 000*3 001*2 000*14 # HCLens: {0:1, 8:1}
			> 1*214 0*2 1*41 0   # HLits: {*:8}
			<0*4 X:f00f          # ???
		`),
		inIdx: 42,
		errf:  "IsCorrupted",
	}, {
		// Complete HCLenTree, complete HLitTree, empty HDistTree,
		// no EOB symbol.
		name: "HuffmanTree14",
		input: db(`<<<
			< 1 10               # Last, dynamic block
			< D5:0 D5:0 D4:15    # HLit: 257, HDist: 1, HCLen: 19
			< 000*3 001*2 000*14 # HCLens: {0:1, 8:1}
			> 1*256 0*2          # HLits: {*:8}, HDists: {}
			> 00000000 11111111  # Compressed data
		`),
		output: dh("00ff"),
		inIdx:  44,
		outIdx: 2,
		errf:   "IsUnexpectedEOF",
	}, {
		// Complete HCLenTree, complete HLitTree, empty HDistTree.
		name: "HuffmanTree15",
		input: db(`<<<
			< 1 10               # Last, dynamic block
			< D5:0 D5:3 D4:15    # HLit: 257, HDist: 4, HCLen: 19
			< 000*3 001*2 000*14 # HCLens: {0:1, 8:1}
			> 0 1*256 0*4        # HLits: {*:8}, HDists: {}
			> 00000000 11111111  # Compressed data
		`),
		output: dh("01"),
		inIdx:  44,
		outIdx: 1,
	}, {
		// Complete HCLenTree, complete HLitTree, degenerate HDistTree,
		// use valid HDist symbol.
		name: "HuffmanTree16",
		input: db(`<<<
			< 0 00 0*5                 # Non-last, raw block, padding
			< H16:0001 H16:fffe        # RawSize: 1
			X:7a                       # Raw data

			< 1 10                     # Last, dynamic block
			< D5:1 D5:0 D4:15          # HLit: 258, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 0*256 1*3                # HLits: {256:1, 257:1}, HDists: {0:1}
			> 1 0*2                    # Compressed data
		`),
		output: dh("7a7a7a7a"),
		inIdx:  48,
		outIdx: 4,
	}, {
		// Complete HCLenTree, degenerate HLitTree, degenerate HDistTree.
		name: "HuffmanTree17",
		input: db(`<<<
			< 1 10                     # Last, dynamic block
			< D5:0 D5:0 D4:15          # HLit: 257, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 0*256 1*2                # HLits: {256:1}, HDists: {0:1}
			> 0                        # Compressed data
		`),
		inIdx: 42,
	}, {
		// Complete HCLenTree, degenerate HLitTree, empty HDistTree.
		name: "HuffmanTree18",
		input: db(`<<<
			< 1 10                     # Last, dynamic block
			< D5:0 D5:0 D4:15          # HLit: 257, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 0*256 1 0                # HLits: {256:1}, # HDists: {}
			> 0                        # Compressed data
		`),
		inIdx: 42,
	}, {
		// Complete HCLenTree, complete HLitTree, empty HDistTree,
		// spanning zero repeater symbol.
		name: "HuffmanTree19",
		input: db(`<<<
			< 1 10                           # Last, dynamic block
			< D5:29 D5:29 D4:15              # HLit: 286, HDist: 30, HCLen: 19
			< 011 000 011 001 000*13 010 000 # HCLens: {0:1, 1:2, 16:3, 18:3}
			> 10 0*255 10 111 <D7:48         # HLits: {0:1, 256:1}, HDists: {}
			> 1                              # Compressed data
		`),
		inIdx: 43,
	}, {
		// Complete HCLenTree, use last repeater on non-zero code.
		name: "HuffmanTree20",
		input: db(`<<<
			< 1 10           # Last, dynamic block
			< D5:0 D5:0 D4:8 # HLit: 257, HDist: 1, HClen: 12
			# HCLens: {0:2, 4:2, 16:2, 18:2}
			< 010 000 010*2 000*7 010
			# HLits: {0-14:4, 256:4}, HDists: {}
			> 01*12 10 <D2:0 11 <D7:127 11 <D7:92 01 00
			# Compressed data
			> 0000 0001 0010 1111
		`),
		output: dh("000102"),
		inIdx:  15,
		outIdx: 3,
	}, {
		// Complete HCLenTree, use last repeater on zero code.
		name: "HuffmanTree21",
		input: db(`<<<
			< 1 10           # Last, dynamic block
			< D5:0 D5:0 D4:8 # HLit: 257, HDist: 1, HClen: 12
			# HCLens: {0:2, 4:2, 16:2, 18:2}
			< 010 000 010*2 000*7 010
			# HLits: {241-256:4}, HDists: {}
			> 00 10 <D2:3 11 <D7:127 11 <D7:85 01*16 00
			# Compressed data
			> 0000 0001 0010 1111
		`),
		output: dh("f1f2f3"),
		inIdx:  16,
		outIdx: 3,
	}, {
		// Complete HCLenTree, use last repeater without first code.
		name: "HuffmanTree22",
		input: db(`<<<
			< 1 10           # Last, dynamic block
			< D5:0 D5:0 D4:8 # HLit: 257, HDist: 1, HClen: 12
			# HCLens: {0:2, 4:2, 16:2, 18:2}
			< 010 000 010*2 000*7 010
			# HLits: {???}, HDists: {???}
			> 10 <D2:3 11 <D7:127 11 <D7:86 01*16 00
			# ???
			> 0000 0001 0010 1111
		`),
		inIdx: 7,
		errf:  "IsCorrupted",
	}, {
		// Complete HCLenTree with length codes, complete HLitTree,
		// empty HDistTree.
		name: "HuffmanTree23",
		input: db(`<<<
			< 1 10                     # Last, dynamic block
			< D5:29 D5:0 D4:15         # HLit: 286, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 0*256 1 0*27 1 0*2       # HLits: {256:1, 284:1}, HDists: {}
			> 0                        # Compressed data
		`),
		inIdx: 46,
	}, {
		// Complete HCLenTree, complete HLitTree, degenerate HDistTree,
		// use valid HLit symbol 284 with count 31.
		name: "HuffmanTree24",
		input: db(`<<<
			< 0 00 0*5                 # Non-last, raw block, padding
			< H16:0001 H16:fffe        # RawSize: 1
			X:00                       # Raw data

			< 1 10                     # Last, dynamic block
			< D5:29 D5:0 D4:15         # HLit: 286, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 0*256 1 0*27 1 0 1       # HLits: {256:1, 284:1}, HDists: {0:1}
			> 1 <D5:31 0*2             # Compressed data
		`),
		output: db("<<< X:00*259"),
		inIdx:  53,
		outIdx: 259,
	}, {
		// Complete HCLenTree, complete HLitTree, degenerate HDistTree,
		// use valid HLit symbol 285.
		name: "HuffmanTree25",
		input: db(`<<<
			< 0 00 0*5                 # Non-last, raw block, padding
			< H16:0001 H16:fffe        # RawSize: 1
			X:00                       # Raw data

			< 1 10                     # Last, dynamic block
			< D5:29 D5:0 D4:15         # HLit: 286, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 0*256 1 0*28 1*2         # HLits: {256:1, 285:1}, HDists: {0:1}
			> 1 0*2                    # Compressed data
		`),
		output: db("<<< X:00*259"),
		inIdx:  52,
		outIdx: 259,
	}, {
		// Complete HCLenTree, complete HLitTree, degenerate HDistTree,
		// use valid HLit and HDist symbols.
		name: "HuffmanTree26",
		input: db(`<<<
			< 0 10            # Non-last, dynamic block
			< D5:1 D5:2 D4:14 # HLit: 258, HDist: 3, HCLen: 18
			# HCLens: {0:3, 1:3, 2:2, 3:2, 18:2}
			< 000*2 010 011 000*9 010 000 010 000 011
			# HLits: {97:3, 98:3, 99:2, 256:2, 257:2}, HDists: {2:1}
			> 10 <D7:86 01 01 00 10 <D7:127 10 <D7:7 00 00 110 110 111
			# Compressed data
			> 110 111 00 10 0 01

			< 1 00 0*3          # Last, raw block, padding
			< H16:0000 H16:ffff # RawSize: 0
		`),
		output: []byte("abcabc"),
		inIdx:  21,
		outIdx: 6,
	}, {
		// Valid short distance match.
		name: "DistanceMatch0",
		input: db(`<<<
			< 0 00 0*5          # Non-last, raw block, padding
			< H16:0001 H16:fffe # RawSize: 1
			X:0f                # Raw data

			< 1 01         # Last, fixed block
			> 0000001 D5:0 # Length: 3, Distance: 1
			> 0000000      # EOB marker
		`),
		output: db("<<< X:0f0f0f0f"),
		inIdx:  9,
		outIdx: 4,
	}, {
		// Valid long distance match.
		name: "DistanceMatch1",
		input: db(`<<<
			< 0 00 0*5                              # Non-last, raw block, padding
			< H16:8000 H16:7fff                     # RawSize: 32768
			X:0f1e2d3c4b5a69788796a5b4c3d2e1f0*2048 # Raw data

			< 1 01                     # Last, fixed block
			> 0000001 D5:29 <H13:1fff  # Length: 3, Distance: 32768
			> 11000101 D5:29 <H13:1fff # Length: 258, Distance: 32768
			> 0000000                  # EOB marker
		`),
		output: db(`<<<
			X:0f1e2d3c4b5a69788796a5b4c3d2e1f0*2048
			X:0f1e2d3c4b5a69788796a5b4c3d2e1f0*16
			X:0f1e2d3c4b
		`),
		inIdx:  32781,
		outIdx: 33029,
	}, {
		// Invalid long distance match with not enough data.
		name: "DistanceMatch2",
		input: db(`<<<
			< 0 00 0*5                              # Non-last, raw block, padding
			< H16:7fff H16:8000                     # RawSize: 32767
			X:0f1e2d3c4b5a69788796a5b4c3d2e1f0*2047 # Raw data
			X:0f1e2d3c4b5a69788796a5b4c3d2e1

			< 1 01                     # Last, fixed block
			> 0000001 D5:29 <H13:1fff  # Length: 3, Distance: 32768
			> 0000000                  # EOB marker
		`),
		output: db(`<<<
			X:0f1e2d3c4b5a69788796a5b4c3d2e1f0*2047
			X:0f1e2d3c4b5a69788796a5b4c3d2e1
		`),
		inIdx:  32776,
		outIdx: 32767,
		errf:   "IsCorrupted",
	}, {
		// Invalid short distance match with no data.
		name: "DistanceMatch3",
		input: db(`<<<
			< 1 01         # Last, fixed block
			> 0000001 D5:0 # Length: 3, Distance: 1
			> 0000000      # EOB marker
		`),
		inIdx: 2,
		errf:  "IsCorrupted",
	}, {
		// Invalid long distance match with no data.
		name: "DistanceMatch4",
		input: db(`<<<
			< 1 01                    # Last, fixed block
			> 0000001 D5:29 <H13:1fff # Length: 3, Distance: 32768
			> 0000000                 # EOB marker
		`),
		inIdx: 4,
		errf:  "IsCorrupted",
	}, {
		// Large HLitTree caused a panic.
		name: "Issue3815",
		input: db(`<<<
			< 0 10             # Non-last, dynamic block
			< D5:31 D5:30 D4:7 # HLit: 288, HDist: 31, HCLen: 11

			# ???
			<0011011
			X:e75e1cefb3555877b656b543f46ff2d2e63d99a0858c48ebf8da83042a75c4f8
			X:0f1211b9b44b09a0be8b914c
		`),
		inIdx: 3,
		errf:  "IsCorrupted",
	}, {
		// Incomplete HCLenTree causes a panic.
		name: "Issue5915",
		input: db(`<<<
			< 1 10             # Last, dynamic block
			< 10000 11001 1010 # HLit: 273, HDist: 26, HCLen: 14

			# HCLens: {...}, this is valid
			< 101 101 110 100 011 011 100 010 100 011 101 100 000 110

			# HLits: {...}, this is invalid
			> 11110 <D3:7 11100 111111 <D7:10 011 100 00 1010 11100 1010 100
			  1010 00 00 1101 100 00 1100 00 011 010 011 011 1100 100
			  11101 <D2:0 1101 100 00 011 1101 00 1101 1100 1010 1100 1100 100
			  1100 11101 <D2:1 100 1010 1010 1100 010 11101 <D2:3 00 1101
			  11101 <D00:0 1100 1101 11110 <D3:2 00 011 100 100 1010 1100 00 100
			  100 1100 00 010 011 011 00 00 011 011 00 00 00 010 00 1100 00 010
			  010 00 011 011 100 011 100 00 011 011 111111 <D7:119 1101 1011
			  1011 010 010 010 00 011 011 00 011 00

			# HDists: {...}, this is invalid
			> 100 00 100 100 11100 100 11110 <D3:0 100 010 010 010 1011 1011
			  111110 010 1011 010 1011 010 1011 010 1011 010
		`),
		inIdx: 61,
		errf:  "IsCorrupted",
	}, {
		// Incomplete HCLenTree causes a panic.
		name: "Issue5962",
		input: db(`<<<
			< 0 10             # Non-last, dynamic block
			< 10101 10011 1000 # HLit: 278, HDist: 20, HCLen: 12
			# HCLens: {0:7, 5:7, 6:6, 9:3, 10:7, 11:1, 16:5, 18:6}
			< 101 000 110 111 000*2 011 110 111*2 001 000
		`),
		inIdx: 7,
		errf:  "IsCorrupted",
	}, {
		// Over-subscribed HCLenTree caused a hang.
		name: "Issue10426",
		input: db(`<<<
			< 0 10                  # Non-last, dynamic block
			< D5:6 D5:12 D4:2       # HLit: 263, HDist: 13, HCLen: 6
			< 101 100*2 011 010 001 # HCLens: {0:3, 7:1, 8:2, 16:5, 17:4, 18:4}, invalid
			<01001 X:4d4b070000ff2e2eff2e2e2e2e2eff # ???
		`),
		inIdx: 5,
		errf:  "IsCorrupted",
	}, {
		// Empty HDistTree unexpectedly led to an error.
		name: "Issue11030",
		input: db(`<<<
			< 1 10            # Last, dynamic block
			< D5:0 D5:0 D4:14 # HLit: 257, HDist: 1, HCLen: 18
			# HCLens: {0:1, 1:4, 2:2, 16:3, 18:4}
			< 011 000 100 001 000*11 010 000 100
			# HLits: {253:2, 254:2, 255:2, 256:2}
			> 0 1111 <D7:112 1111 <D7:111 0 0 0 0 0 0 0 10 10 10 10
			# HDists: {}
			> 0
			# Compressed data
			> 11
		`),
		inIdx: 14,
	}, {
		// Empty HDistTree unexpectedly led to an error.
		name: "Issue11033",
		input: db(`<<<
			< 1 10           # Last, dynamic block
			< D5:0 D5:0 D4:8 # HLit: 257, HDist: 1, HCLen: 12
			# HCLens: {0:2, 4:3, 5:2, 6:3, 17:3, 18:3}
			< 000 011*2 010 000*3 011 000 010 000 011
			# HLits: {...}
			> 01 110 100 101 00 00 101 111 1010000 01 110 110 01 111 0100000
			  101 00 100 01 00 00 100 01 01 111 0001000 01 111 1000000 01 110
			  010 100 00 01 110 010 01 00 00 100 110 001 100 111 0100000 01
			  111 0110000 01 00 01 111 0001010 100 110 011 01 110 110 101 00
			  101 110 011 101 110 001 101 111 0001000 101 100
			# HDists: {}
			> 00
			# Compressed data
			> 10001 0000 0000 10011 0001 0001 10000 0011 10111 111010 0100
			  0011 0100 01110 0010 111000 10010 10110 11000 111100 10101
			  111111 111001 10100 11001 11010 0010 01111 111101 111110 0101
			  11011 0101 111011 0110
		`),
		output: dh("" +
			"3130303634342068652e706870005d05355f7ed957ff084a90925d19e3ebc6d0" +
			"c6d7",
		),
		inIdx:  57,
		outIdx: 34,
	}}

	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			rd, err := NewReader(bytes.NewReader(v.input), nil)
			if err != nil {
				t.Fatalf("unexpected NewReader error: %v", err)
			}
			output, err := ioutil.ReadAll(rd)
			if cerr := rd.Close(); cerr != nil {
				err = cerr
			}

			if got, want, ok := testutil.BytesCompare(output, v.output); !ok {
				t.Errorf("output mismatch:\ngot  %s\nwant %s", got, want)
			}
			if rd.InputOffset != v.inIdx {
				t.Errorf("input offset mismatch: got %d, want %d", rd.InputOffset, v.inIdx)
			}
			if rd.OutputOffset != v.outIdx {
				t.Errorf("output offset mismatch: got %d, want %d", rd.OutputOffset, v.outIdx)
			}
			if v.errf != "" && !errFuncs[v.errf](err) {
				t.Errorf("mismatching error:\ngot %v\nwant %s(err) == true", err, v.errf)
			} else if v.errf == "" && err != nil {
				t.Errorf("unexpected error: got %v", err)
			}

			// If the zcheck flag is set, then we verify that the test vectors
			// themselves are consistent with what the C zlib library outputs.
			// To do that, we use the python wrapper around the library.
			if *zcheck {
				output, err := pyDecompress(v.input)
				if got, want := bool(v.errf == ""), bool(err == nil); got != want {
					t.Errorf("pass mismatch: got %v, want %v", got, want)
				}
				if got, want, ok := testutil.BytesCompare(output, v.output); !ok && err == nil {
					t.Errorf("output mismatch:\ngot  %s\nwant %s", got, want)
				}
			}
		})
	}
}

// TestReaderEarlyEOF tests that Reader returns io.EOF eagerly when possible.
// There are two situations when it is unable to do so:
//	* There is an non-last, empty, raw block at the end of the stream.
//	Flushing semantics dictate that we must return at that point in the stream
//	prior to knowing whether the end of the stream has been hit or not.
//	* We previously returned from Read because the internal dictionary was full
//	and it so happens that there is no more data to read. This is rare.
func TestReaderEarlyEOF(t *testing.T) {
	const maxSize = 1 << 18
	const dampRatio = 32 // Higher value means more trials

	data := make([]byte, maxSize)
	for i := range data {
		data[i] = byte(i)
	}

	// generateStream generates a DEFLATE stream that decompresses to n bytes
	// of arbitrary data. If flush is set, then a final Flush is called at the
	// very end of the stream.
	var wrBuf bytes.Buffer
	wr, _ := flate.NewWriter(nil, flate.BestSpeed)
	generateStream := func(n int, flush bool) []byte {
		wrBuf.Reset()
		wr.Reset(&wrBuf)
		wr.Write(data[:n])
		if flush {
			wr.Flush()
		}
		wr.Close()
		return wrBuf.Bytes()
	}

	// readStream reads all the data and reports whether an early EOF occurred.
	var rd Reader
	rdBuf := make([]byte, 2111)
	readStream := func(data []byte) (bool, error) {
		rd.Reset(bytes.NewReader(data))
		for {
			n, err := rd.Read(rdBuf)
			if err != nil {
				if err == io.EOF {
					err = nil
				}
				return n > 0, err
			}
		}
	}

	// There is no guarantee that early io.EOF occurs for all DEFLATE streams,
	// but it should occur for most cases.
	var numEarly, numTotal int
	for i := 0; i < maxSize; i += 1 + i/dampRatio {
		earlyEOF, err := readStream(generateStream(i, false))
		if err != nil {
			t.Errorf("unexpected Read error: %v", err)
		}
		if earlyEOF {
			numEarly++
		}
		numTotal++
	}
	got := 100 * float64(numEarly) / float64(numTotal)
	if want := 95.0; got < want {
		t.Errorf("too few early EOFs: %0.1f%% < %0.1f%%", got, want)
	}

	// If there is a flush block at the end of all the data, an early io.EOF
	// is never possible. Check for this case.
	for i := 0; i < maxSize; i += 1 + i/dampRatio {
		earlyEOF, err := readStream(generateStream(i, true))
		if err != nil {
			t.Errorf("unexpected Read error: %v", err)
		}
		if earlyEOF {
			t.Errorf("size: %d, unexpected early EOF with terminating flush", i)
		}
	}
}

func TestReaderReset(t *testing.T) {
	const data = "\x00\x0c\x00\xf3\xffhello, world\x01\x00\x00\xff\xff"

	var rd Reader
	if err := rd.Close(); err != nil {
		t.Errorf("unexpected Close error: %v", err)
	}

	rd.Reset(strings.NewReader("garbage"))
	if _, err := ioutil.ReadAll(&rd); !errors.IsCorrupted(err) {
		t.Errorf("mismatching Read error: got %v, want IsCorrupted(err) == true", err)
	}
	if err := rd.Close(); !errors.IsCorrupted(err) {
		t.Errorf("mismatching Close error: got %v, want IsCorrupted(err) == true", err)
	}

	rd.Reset(strings.NewReader(data))
	if _, err := ioutil.ReadAll(&rd); err != nil {
		t.Errorf("unexpected Read error: %v", err)
	}
	if err := rd.Close(); err != nil {
		t.Errorf("unexpected Close error: %v", err)
	}
}

func BenchmarkDecode(b *testing.B) {
	runBenchmarks(b, func(b *testing.B, data []byte, lvl int) {
		b.StopTimer()
		b.ReportAllocs()

		buf := new(bytes.Buffer)
		wr, _ := flate.NewWriter(buf, lvl)
		wr.Write(data)
		wr.Close()

		br := new(bytes.Reader)
		rd := new(Reader)

		b.SetBytes(int64(len(data)))
		b.StartTimer()
		for i := 0; i < b.N; i++ {
			br.Reset(buf.Bytes())
			rd.Reset(br)

			n, err := io.Copy(ioutil.Discard, rd)
			if n != int64(len(data)) || err != nil {
				b.Fatalf("Copy() = (%d, %v), want (%d, nil)", n, err, len(data))
			}
			if err := rd.Close(); err != nil {
				b.Fatalf("Close() = %v, want nil", err)
			}
		}
	})
}
