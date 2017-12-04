// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"github.com/dsnet/compress/internal/errors"
	"github.com/dsnet/compress/internal/testutil"
)

func TestReader(t *testing.T) {
	db := testutil.MustDecodeBitGen

	errFuncs := map[string]func(error) bool{
		"IsUnexpectedEOF": func(err error) bool { return err == io.ErrUnexpectedEOF },
		"IsCorrupted":     errors.IsCorrupted,
		"IsDeprecated":    errors.IsDeprecated,
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
		name:  "EmptyOutput",
		input: db(`>>> > "BZh9" H48:177245385090 H32:00000000`),
		inIdx: 14,
	}, {
		name: "EmptyOutput9S",
		input: db(`>>> >
			"BZh1" H48:177245385090 H32:00000000
			"BZh2" H48:177245385090 H32:00000000
			"BZh3" H48:177245385090 H32:00000000
			"BZh4" H48:177245385090 H32:00000000
			"BZh5" H48:177245385090 H32:00000000
			"BZh6" H48:177245385090 H32:00000000
			"BZh7" H48:177245385090 H32:00000000
			"BZh8" H48:177245385090 H32:00000000
			"BZh9" H48:177245385090 H32:00000000
		`),
		inIdx: 14 * 9, outIdx: 0,
	}, {
		name:  "InvalidStreamMagic",
		input: db(`>>> > "XX"`),
		inIdx: 2, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		name:  "InvalidVersion",
		input: db(`>>> > "BZX1"`),
		inIdx: 3, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		name:  "DeprecatedVersion",
		input: db(`>>> > "BZ01"`),
		inIdx: 3, outIdx: 0,
		errf: "IsDeprecated",
	}, {
		name:  "InvalidLevel",
		input: db(`>>> > "BZh0"`),
		inIdx: 4, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		name:  "InvalidBlockMagic",
		input: db(`>>> > "BZh9" H48:000000000000`),
		inIdx: 10, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		name:  "DeprecatedRandomization",
		input: db(`>>> > "BZh9" H48:314159265359 H32:8e9a7706 1 H24:0`),
		inIdx: 15, outIdx: 0,
		errf: "IsDeprecated",
	}, {
		name:  "Truncated1",
		input: db(`>>> "BZh9"`),
		inIdx: 4, outIdx: 0,
		errf: "IsUnexpectedEOF",
	}, {
		name:  "Truncated2",
		input: db(`>>> > "BZh9" H40:3141592653`),
		inIdx: 8, outIdx: 0,
		errf: "IsUnexpectedEOF",
	}, {
		name:  "Truncated3",
		input: db(`>>> > "BZh9" H48:314159265359`),
		inIdx: 10, outIdx: 0,
		errf: "IsUnexpectedEOF",
	}, {
		name:  "Truncated4",
		input: db(`>>> > "BZh9" H48:314159265359 H16:8e9a`),
		inIdx: 10, outIdx: 0,
		errf: "IsUnexpectedEOF",
	}, {
		name:  "Truncated5",
		input: db(`>>> > "BZh9" H48:314159265359 H32:8e9a7706`),
		inIdx: 14, outIdx: 0,
		errf: "IsUnexpectedEOF",
	}, {
		name:  "Truncated6",
		input: db(`>>> > "BZh9" H48:314159265359 H32:8e9a7706 0 H24:3`),
		inIdx: 18, outIdx: 0,
		errf: "IsUnexpectedEOF",
	}, {
		name:  "Truncated7",
		input: db(`>>> > "BZh9"  H48:314159265359 H32:8e9a7706 0 H24:3 < H16:00d4 H16:1003`),
		inIdx: 22, outIdx: 0,
		errf: "IsUnexpectedEOF",
	}, {
		name: "Truncated8",
		input: db(`>>>
			"BZh9"
			> H48:314159265359 H32:8e9a7706 0 H24:3
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
		`),
		inIdx: 33, outIdx: 0,
		errf: "IsUnexpectedEOF",
	}, {
		name: "Truncated9",
		input: db(`>>>
			"BZh9"
			> H48:314159265359 H32:8e9a7706 0 H24:3
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
			< 1101 000 100 000 100 0111 010 010
		`),
		inIdx: 39, outIdx: 0,
		errf: "IsUnexpectedEOF",
	}, {
		name: "Truncated10",
		input: db(`>>>
			"BZh9"
			> H48:314159265359 H32:8e9a7706 0 H24:3
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111
		`),
		output: []byte("Hello, world!"),
		inIdx:  41, outIdx: 13,
		errf: "IsUnexpectedEOF",
	}, {
		name: "HelloWorld",
		input: db(`>>>
			"BZh9"
			> H48:314159265359 H32:8e9a7706 0 H24:3
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111
			> H48:177245385090 H32:8e9a7706
		`),
		output: []byte("Hello, world!"),
		inIdx:  51, outIdx: 13,
	}, {
		name: "HelloWorld2B",
		input: db(`>>>
			"BZh9"

			( # Two blocks
				> H48:314159265359 H32:8e9a7706 0 H24:3
				< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
				> D3:2 D15:1 0
				> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
				> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
				< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111
			)*2

			> H48:177245385090 H32:93ae990b
		`),
		output: db(`>>> "Hello, world!"*2`),
		inIdx:  51*2 - 4 - 10, outIdx: 13 * 2,
	}, {
		name: "HelloWorld2S",
		input: db(`>>>
			( # Two streams
				"BZh9"

				> H48:314159265359 H32:8e9a7706 0 H24:3
				< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
				> D3:2 D15:1 0
				> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
				> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
				< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111

				> H48:177245385090 H32:8e9a7706
			)*2
		`),
		output: db(`>>> "Hello, world!"*2`),
		inIdx:  51 * 2, outIdx: 13 * 2,
	}, {
		name: "Banana0",
		input: db(`>>>
			> "BZh1" H48:314159265359 H32:87f465d8 0 H24:0
			< H16:0050 H16:0004 H16:4002
			> D3:2 D15:1 0 D5:2 0 10100 0 1111110 10100 D5:3 0 0 110 0 0
			< 1111 0 01 0 0 01 011
			> H48:177245385090 H32:87f465d8
		`),
		output: []byte("Banana"),
		inIdx:  42, outIdx: 6,
	}, {
		name: "Banana1",
		input: db(`>>>
			> "BZh1" H48:314159265359 H32:71d297e8 0 H24:1
			< H16:0050 H16:0004 H16:4002
			> D3:2 D15:1 0 D5:2 0 10100 0 1111110 10100 D5:3 0 0 110 0 0
			< 1111 0 01 0 0 01 011
			> H48:177245385090 H32:71d297e8
		`),
		output: []byte("aBanan"),
		inIdx:  42, outIdx: 6,
	}, {
		name: "Banana2",
		input: db(`>>>
			> "BZh1" H48:314159265359 H32:21185406 0 H24:2
			< H16:0050 H16:0004 H16:4002
			> D3:2 D15:1 0 D5:2 0 10100 0 1111110 10100 D5:3 0 0 110 0 0
			< 1111 0 01 0 0 01 011
			> H48:177245385090 H32:21185406
		`),
		output: []byte("anaBan"),
		inIdx:  42, outIdx: 6,
	}, {
		name: "Banana3",
		input: db(`>>>
			> "BZh1" H48:314159265359 H32:be853f46 0 H24:3
			< H16:0050 H16:0004 H16:4002
			> D3:2 D15:1 0 D5:2 0 10100 0 1111110 10100 D5:3 0 0 110 0 0
			< 1111 0 01 0 0 01 011
			> H48:177245385090 H32:be853f46
		`),
		output: []byte("ananaB"),
		inIdx:  42, outIdx: 6,
	}, {
		name: "Banana4",
		input: db(`>>>
			> "BZh1" H48:314159265359 H32:35a020df 0 H24:4
			< H16:0050 H16:0004 H16:4002
			> D3:2 D15:1 0 D5:2 0 10100 0 1111110 10100 D5:3 0 0 110 0 0
			< 1111 0 01 0 0 01 011
			> H48:177245385090 H32:35a020df
		`),
		output: []byte("naBana"),
		inIdx:  42, outIdx: 6,
	}, {
		name: "Banana5",
		input: db(`>>>
			> "BZh1" H48:314159265359 H32:b599e6fc 0 H24:5
			< H16:0050 H16:0004 H16:4002
			> D3:2 D15:1 0 D5:2 0 10100 0 1111110 10100 D5:3 0 0 110 0 0
			< 1111 0 01 0 0 01 011
			> H48:177245385090 H32:b599e6fc
		`),
		output: []byte("nanaBa"),
		inIdx:  42, outIdx: 6,
	}, {
		// This is invalid since the BWT pointer exceeds the block size.
		name: "Banana6",
		input: db(`>>>
			> "BZh1" H48:314159265359 H32:87f465d8 0 H24:6
			< H16:0050 H16:0004 H16:4002
			> D3:2 D15:1 0 D5:2 0 10100 0 1111110 10100 D5:3 0 0 110 0 0
			< 1111 0 01 0 0 01 011
			> H48:177245385090 H32:87f465d8
		`),
		inIdx: 42 - 10, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		// There must be between 2..6 trees, inclusive. This test uses only 1.
		name: "MinTrees",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:8e9a7706 0 H24:3
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:1 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111
			> H48:177245385090 H32:8e9a7706
		`),
		inIdx: 28, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		// Define more trees than allowed. The test uses 7.
		name: "MaxTrees",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:8e9a7706 0 H24:3
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:7 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			>(D5:4 0 0 0 0 0 0 0 0 110 0 0 0)*6
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111
			> H48:177245385090 H32:8e9a7706
		`),
		inIdx: 28, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		// Define more trees and selectors than actually used.
		name: "SuboptimalTrees",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:8e9a7706 0 H24:3
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:6 D15:12 111110 11110 1110 110 10 0 111110 11110 1110 110 10 0
			>(D5:4 0 0 0 0 0 0 0 0 110 0 0 0)*5
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111
			> H48:177245385090 H32:8e9a7706
		`),
		output: []byte("Hello, world!"),
		inIdx:  66, outIdx: 13,
	}, {
		// Do not define any tree selectors. This should fail when decoding
		// the prefix codes later on.
		name: "MinTreeSels",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:8e9a7706 0 H24:3
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:2 D15:0 # No selectors defined
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111
			> H48:177245385090 H32:8e9a7706
		`),
		inIdx: 35, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		// Define up to 32767 tree selectors, even though only 1 is used.
		name: "MaxTreeSels",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:8e9a7706 0 H24:3
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:2 D15:32767 0*32767 # Define all selectors
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111
			> H48:177245385090 H32:8e9a7706
		`),
		output: []byte("Hello, world!"),
		inIdx:  4147, outIdx: 13,
	}, {
		name: "InvalidTreeSels1",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:8e9a7706 0 H24:3
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:2 D15:1 110 # Select tree2, which does not exist
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111
			> H48:177245385090 H32:8e9a7706
		`),
		inIdx: 30, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		name: "InvalidTreeSels2",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:8e9a7706 0 H24:3
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:6 D15:1 111111 # Select tree6, which is invalid
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			>(D5:4 0 0 0 0 0 0 0 0 110 0 0 0)*5
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111
			> H48:177245385090 H32:8e9a7706
		`),
		inIdx: 31, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		name: "JunkPadding",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:b1f7404b 0 H24:0
			< H16:0001 H16:0001
			> D3:2 D15:1 0 D5:2 0 0 110 D5:2 0 0 110
			< 01 0
			> H48:177245385090 H32:b1f7404b 10101 # Non-zero padding bits
		`),
		output: []byte{0x00},
		inIdx:  37, outIdx: 1,
	}, {
		name: "MinSymMap",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:b1f7404b 0 H24:0
			< H16:0001 H16:0001 # Only one symbol used
			> D3:2 D15:1 0
			>(D5:2 0 0 110)*2
			< 01 0
			> H48:177245385090 H32:b1f7404b
		`),
		output: []byte{0x00},
		inIdx:  37, outIdx: 1,
	}, {
		// This block satisfies the minimum of 3 symbols for prefix encoding.
		// The data section terminates immediately with an EOF symbol.
		// However, this is still invalid since a BWT pointer of 0 >= 0.
		name: "EmptyBlock",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:00000000 0 H24:0 # BWT pointer of 0
			< H16:0001 H16:0001 # Only one symbol used
			> D3:2 D15:1 0
			>(D5:2 0 0 110)*2
			< 0 # Data ends immediately with EOF
			> H48:177245385090 H32:00000000
		`),
		inIdx: 27, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		// The high-order symbol map says that all groups have symbols,
		// but only the first group indicates any symbols are set.
		name: "SuboptimalSymMap1",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:b1f7404b 0 H24:0
			< H16:ffff H16:0001 H16:0000*15 # All groups used, only one symbol used
			> D3:2 D15:1 0
			>(D5:2 0 0 110)*2
			< 01 0
			> H48:177245385090 H32:b1f7404b
		`),
		output: []byte{0x00},
		inIdx:  67, outIdx: 1,
	}, {
		// The symbol map declares that all symbols are used, even though
		// only one is actually used.
		name: "SuboptimalSymMap2",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:b1f7404b 0 H24:0
			< H16:ffff*17 # All symbols used
			> D3:2 D15:1 0
			> D5:2 0 10101010101010100 0*255 1111111111111111110
			> D5:9 0*4 110 0*253
			< 01 0
			> H48:177245385090 H32:b1f7404b
		`),
		output: []byte{0x00},
		inIdx:  135, outIdx: 1,
	}, {
		// It is impossible for the format to encode a block with no symbols
		// since at least one symbol must exist for the EOF symbol.
		name: "InvalidSymMap",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:b1f7404b 0 H24:0
			< H16:0000 # Need at least one symbol
		`),
		inIdx: 20, outIdx: 0,
		errf: "IsCorrupted", // Should not be IsUnexpectedEOF
	}, {
		name: "InvalidBlockChecksum",
		input: db(`>>>
			"BZh9"
			> H48:314159265359 H32:00000000 0 H24:3 # Bad checksum
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111
			> H48:177245385090 H32:8e9a7706
		`),
		output: []byte("Hello, world!"),
		inIdx:  51 - 10, outIdx: 13,
		errf: "IsCorrupted",
	}, {
		name: "InvalidStreamChecksum",
		input: db(`>>>
			"BZh9"
			> H48:314159265359 H32:8e9a7706 0 H24:3
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111
			> H48:177245385090 H32:00000000 # Bad checksum
		`),
		output: []byte("Hello, world!"),
		inIdx:  51, outIdx: 13,
		errf: "IsCorrupted",
	}, {
		// RLE1 stage with maximum repeater length.
		name: "RLE1-1",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:e1fac440 0 H24:0
			< H16:8010 H16:0002 H16:8000
			> D3:2 D15:1 0
			> D5:2 0 100 11110 10100
			> D5:2 0 0 0 0
			< 0 0 01 01 111 # Pre-RLE1: "AAAA\xff"
			> H48:177245385090 H32:e1fac440
		`),
		output: db(`>>> X:41*259`),
		inIdx:  41, outIdx: 259,
	}, {
		// RLE1 stage with minimum repeater length.
		name: "RLE1-2",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:e16e6571 0 H24:4
			< H16:0011 H16:0001 H16:0002
			> D3:2 D15:1 0
			> D5:2 0 100 11110 10100
			> D5:2 0 0 0 0
			< 0 01 01 0 111 # Pre-RLE1: "AAAA\x00"
			> H48:177245385090 H32:e16e6571
		`),
		output: db(`>>> X:41*4`),
		inIdx:  41, outIdx: 4,
	}, {
		// RLE1 stage with missing repeater value.
		name: "RLE1-3",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:e16e6571 0 H24:3
			< H16:0010 H16:0002
			> D3:2 D15:1 0
			>(D5:2 0 0 110)*2
			< 11 01 0 # Pre-RLE1: "AAAA"
			> H48:177245385090 H32:e16e6571
		`),
		output: db(`>>> X:41*4`),
		inIdx:  37 - 10, outIdx: 4,
		errf: "IsCorrupted",
	}, {
		// RLE1 stage with sub-optimal repeater usage.
		name: "RLE1-4",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:f59a903a 0 H24:9
			< H16:0011 H16:0001 H16:0002
			> D3:2 D15:1 0
			> D5:1 0 10100 110 100
			> D5:2 0 0 0 0
			< 01 0 0 0 01 0 111 # Pre-RLE1: "AAAA\x00AAAA\x00"
			> H48:177245385090 H32:f59a903a
		`),
		output: db(`>>> X:41*8`),
		inIdx:  41, outIdx: 8,
	}, {
		// RLE1 stage with sub-optimal repeater usage.
		name: "RLE1-5",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:f59a903a 0 H24:4
			< H16:0011 H16:0002 H16:0002
			> D3:2 D15:1 0
			> D5:3 0 110 110 10100
			> D5:2 0 0 0 0
			< 0 01 01 0 111 # Pre-RLE1: "AAAA\x01AAA"
			> H48:177245385090 H32:f59a903a
		`),
		output: db(`>>> X:41*8`),
		inIdx:  40, outIdx: 8,
	}, {
		name: "RLE2-1",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:6b4f087c 0 H24:000000
			< H16:0040 H16:0006
			> D3:2 D15:1 0
			> D5:1 0 100 100 0
			> D5:2 0 0 0 0
			< 01 0 0 0 0 01 0 01 0 01 01 0 0 0 0 01 111 # a*100k
			> H48:177245385090 H32:6b4f087c
		`),
		output: db(`>>> "a"*2020000`),
		inIdx:  40, outIdx: 2020000,
	}, {
		name: "RLE2-2",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:d175ea9d 0 H24:000000
			< H16:0040 H16:0006
			> D3:2 D15:1 0
			> D5:1 0 100 100 0
			> D5:2 0 0 0 0
			< 0 01 0 0 0 01 0 01 0 01 01 0 0 0 0 01 111 # a*(100k+1)
			> H48:177245385090 H32:d175ea9d
		`),
		inIdx: 40 - 10, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		name: "RLE2-3",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:6b4f087c 0 H24:000000
			< H16:0040 H16:0006
			> D3:2 D15:1 0
			> D5:1 0 100 100 0
			> D5:2 0 0 0 0
			< 0 0 0 0 0 01 0 01 0 01 01 0 0 0 0 01 011 111 # a*(100k-1) b
			> H48:177245385090 H32:6b4f087c
		`),
		output: db(`>>> "a"*2020000`),
		inIdx:  40, outIdx: 2020000,
	}, {
		name: "RLE2-4",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:d175ea9d 0 H24:000000
			< H16:0040 H16:0006
			> D3:2 D15:1 0
			> D5:1 0 100 100 0
			> D5:2 0 0 0 0
			< 0 0 0 0 0 01 0 01 0 01 01 0 0 0 0 01 011 011 111 # a*(100k-1) b a
			> H48:177245385090 H32:d175ea9d
		`),
		inIdx: 40 - 10, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		name: "RLE2-5",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:79235035 0 H24:000000
			< H16:0040 H16:0006
			> D3:2 D15:1 0
			> D5:1 0 100 100 0
			> D5:2 0 0 0 0
			< 0 0 0 0 0 01 0 01 0 01 01 0 0 0 0 01 011 0 011 111 # a*(100k-1) b*2 a
			> H48:177245385090 H32:79235035
		`),
		inIdx: 41 - 10, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		// This input has a sequence of RUNA and RUNB symbols that tries to
		// cause an integer overflow in the numeral decoding.
		name: "RLE2-6",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:6b4f087c 0 H24:000000
			< H16:0040 H16:0006
			> D3:2 D15:1 0
			> D5:1 0 100 100 0
			> D5:2 0 0 0 0
			< 0*32 111
			> H48:177245385090 H32:6b4f087c
		`),
		inIdx: 41 - 10, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		// This is valid, but with suboptimal clen value.
		name: "PrefixBits1",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:b1f7404b 0 H24:0
			< H16:0001 H16:0001
			> D3:2 D15:1 0
			> D5:1 100 0 110 # (1)+1=2 (2)=2 (2)-1=1
			> D5:2 0 0 110
			< 01 0
			> H48:177245385090 H32:b1f7404b
		`),
		output: []byte{0x00},
		inIdx:  37, outIdx: 1,
	}, {
		// This is invalid, clen starts at 0.
		name: "PrefixBits2",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:b1f7404b 0 H24:0
			< H16:0001 H16:0001
			> D3:2 D15:1 0
			> D5:0 10100 0 110 # (0)+2=2 (2)=2 (2)-1=1
			> D5:2 0 0 110
			< 01 0
			> H48:177245385090 H32:b1f7404b
		`),
		inIdx: 25, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		// This is valid, although suboptimal, since clen stays within bounds.
		name: "PrefixBits3",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:b1f7404b 0 H24:0
			< H16:0001 H16:0001
			> D3:2 D15:1 0
			> D5:4 11*3 10*19 11*18 0 0 110 # (4)-3+19-18=2 (2)=2 (2)-1=1
			> D5:2 0 0 110
			< 01 0
			> H48:177245385090 H32:b1f7404b
		`),
		output: []byte{0x00},
		inIdx:  47, outIdx: 1,
	}, {
		// This is invalid since clen temporarily goes over max bits,
		// even though the end value is still "valid".
		name: "PrefixBits4",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:b1f7404b 0 H24:0
			< H16:0001 H16:0001
			> D3:2 D15:1 0
			> D5:4 11*3 10*20 11*19 0 0 110 # (4)-3+20-19=2 (2)=2 (2)-1=1
			> D5:2 0 0 110
			< 01 0
			> H48:177245385090 H32:b1f7404b
		`),
		inIdx: 30, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		// This is invalid since clen temporarily hits zero,
		// even though the end value is still "valid".
		name: "PrefixBits5",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:b1f7404b 0 H24:0
			< H16:0001 H16:0001
			> D3:2 D15:1 0
			> D5:4 11*4 10*20 11*18 0 0 110 # (4)-4+20-18=2 (2)=2 (2)-1=1
			> D5:2 0 0 110
			< 01 0
			> H48:177245385090 H32:b1f7404b
		`),
		inIdx: 26, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		// This is valid since clen starts at max bits and works down.
		name: "PrefixBits6",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:b1f7404b 0 H24:0
			< H16:0001 H16:0001
			> D3:2 D15:1 0
			> D5:20 11*18 0 0 110 # (20)-18=2 (2)=2 (2)-1=1
			> D5:2 0 0 110
			< 01 0
			> H48:177245385090 H32:b1f7404b
		`),
		output: []byte{0x00},
		inIdx:  41, outIdx: 1,
	}, {
		// This is invalid because clen starts at 21, which violates max bits.
		name: "PrefixBits7",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:b1f7404b 0 H24:0
			< H16:0001 H16:0001
			> D3:2 D15:1 0
			> D5:21 11*19 0 0 110 # (21)-19=2 (2)=2 (2)-1=1
			> D5:2 0 0 110
			< 01 0
			> H48:177245385090 H32:b1f7404b
		`),
		inIdx: 25, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		// There are way more prefix symbols in this block than the format
		// even allows. The prefix decoder should detect this cause and
		// report the stream as corrupted.
		name: "MaxPrefixSymbols",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:b1f7404b 0 H24:0
			< H16:0001 H16:0001
			> D3:2 D15:32767 0*32767 # Define all selectors
			> D5:1 0 100 0
			> D5:2 0 0 110
			< H64:0*1000000 11 # 64M symbols
			> H48:177245385090 H32:b1f7404b
		`),
		inIdx: 16622, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		// Use of an over-subscribed tree.
		name: "PrefixTrees1",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:952735b9 0 H24:000000
			< H16:0008 H16:03ff
			> D3:2 D15:1 0
			> D5:5 0 110 0 0 0 0 0 110 0 0 0 0
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
			< 110 0101 1101 0011 1011 0111 000 100 010 110 001
			> H48:177245385090 H32:952735b9
		`),
		output: []byte("03791589269"),
		inIdx:  44, outIdx: 11,
	}, {
		// Use of an under-subscribed tree.
		name: "PrefixTrees2",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:58fdd3b0 0 H24:000000
			< H16:0008 H16:03ff
			> D3:2 D15:1 0
			> D5:5 0 0 0 0 110 0 0 110 0 0 0 0
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
			< 000 100 00111 1101 11011 10111 0101 010 0011 110 01011 001
			> H48:177245385090 H32:58fdd3b0
		`),
		output: []byte("071876222607"),
		inIdx:  45, outIdx: 12,
	}, {
		// Use of an under-subscribed tree and using an invalid symbol.
		name: "PrefixTrees3",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:58fdd3b0 0 H24:000000
			< H16:0008 H16:03ff
			> D3:2 D15:1 0
			> D5:5 0 0 0 0 110 0 0 110 0 0 0 0
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
			< 000 100 00111 1101 11011 10111 0101 010 0011 110 01011 1111 001
			> H48:177245385090 H32:58fdd3b0
		`),
		inIdx: 35, outIdx: 0,
		errf: "IsCorrupted",
	}, {
		// The BWT should be a permutation, but the use of an origin pointer
		// means that the transformation is not a pure bijective function.
		// Thus, there are some inputs where the input is not permuted.
		// This test makes sure we have the same behavior as the C library.
		// No sane encoder should ever output a BWT transform like this.
		name: "NonReversibleBWT",
		input: db(`>>>
			"BZh6"
			> H48:314159265359 H32:01007588 0 H24:000000
			< H16:0040 H16:0006
			> D3:2 D15:1 0
			> D5:3 0 110 110 10100
			> D5:2 0 0 0 0
			< 011 011 0 0 01 0 0 01 0 0 01 0 0 01 0 111
			> H48:177245385090 H32:01007588
		`),
		output: db(`>>> "a"*404`),
		inIdx:  40, outIdx: 404,
	}, {
		// The next "stream" is only a single byte 0x30, which the Reader
		// detects as being truncated since it loads 2 bytes for the magic.
		name:  "Fuzz1",
		input: db(`>>> > "BZh8" H48:177245385090 H32:00000000 X:30`),
		inIdx: 14, outIdx: 0,
		errf: "IsUnexpectedEOF", // Could be IsCorrupted
	}, {
		// Compared to Fuzz1, the next "stream" has 2 bytes 0x3030,
		// which allows the Reader to properly compare with the magic header
		// and reject the stream as invalid.
		name:  "Fuzz2",
		input: db(`>>> > "BZh8" H48:177245385090 H32:00000000 X:3030`),
		inIdx: 16, outIdx: 0,
		errf: "IsCorrupted",
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
			// themselves are consistent with what the C bzip2 library outputs.
			if *zcheck {
				output, err := cmdDecompress(v.input)
				if got, want := bool(v.errf == ""), bool(err == nil); got != want {
					t.Errorf("pass mismatch: got %v, want %v", got, err)
				}
				if got, want, ok := testutil.BytesCompare(output, v.output); !ok && err == nil {
					t.Errorf("output mismatch:\ngot  %s\nwant %s", got, want)
				}
			}
		})
	}
}

func BenchmarkDecode(b *testing.B) {
	runBenchmarks(b, func(b *testing.B, data []byte, lvl int) {
		b.StopTimer()
		b.ReportAllocs()

		buf := new(bytes.Buffer)
		wr, _ := NewWriter(buf, &WriterConfig{Level: lvl})
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
