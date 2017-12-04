// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"testing"

	"github.com/dsnet/compress/internal/testutil"
)

func TestBurrowsWheelerTransform(t *testing.T) {
	vectors := []struct {
		input  []byte // The input test string
		output []byte // Expected output string after BWT
		ptr    int    // The BWT origin pointer
	}{{
		input:  []byte(""),
		output: []byte(""),
		ptr:    -1,
	}, {
		input:  []byte("Hello, world!"),
		output: []byte(",do!lHrellwo "),
		ptr:    3,
	}, {
		input:  []byte("SIX.MIXED.PIXIES.SIFT.SIXTY.PIXIE.DUST.BOXES"),
		output: []byte("TEXYDST.E.IXIXIXXSSMPPS.B..E.S.EUSFXDIIOIIIT"),
		ptr:    29,
	}, {
		input:  []byte("0123456789"),
		output: []byte("9012345678"),
		ptr:    0,
	}, {
		input:  []byte("9876543210"),
		output: []byte("1234567890"),
		ptr:    9,
	}, {
		input:  []byte("The quick brown fox jumped over the lazy dog."),
		output: []byte("kynxederg.l ie hhpv otTu c uwd rfm eb qjoooza"),
		ptr:    9,
	}, {
		input: []byte("" +
			"Mary had a little lamb, its fleece was white as snow" +
			"Mary had a little lamb, its fleece was white as snow" +
			"Mary had a little lamb, its fleece was white as snow" +
			"Mary had a little lamb, its fleece was white as snow" +
			"Mary had a little lamb, its fleece was white as snow" +
			"Mary had a little lamb, its fleece was white as snow" +
			"Mary had a little lamb, its fleece was white as snow" +
			"Mary had a little lamb, its fleece was white as snow" +
			"Nary had a little lamb, its fleece was white as snow"),
		output: []byte("" +
			"dddddddddeeeeeeeeesssssssssyyyyyyyyy,,,,,,,,,eeeeeee" +
			"eeaaaaaaaaassssssssseeeeeeeeesssssssssbbbbbbbbbwwwww" +
			"wwww         hhhhhhhhhlllllllllNMMMMMMMM         www" +
			"wwwwwwmmmmmmmmmeeeeeeeeeaaaaaaaaatttttttttlllllllllc" +
			"cccccccceeeeeeeeelllllllll                  wwwwwwww" +
			"whhhhhhhhh         lllllllll         tttttttttffffff" +
			"fff         aaaaaaaaasssssssssnnnnnnnnnaaaaaaaaatttt" +
			"tttttaaaaaaaaaaaaaaaaaa         iiiiiiiiitttttttttii" +
			"iiiiiiiiiiiiiiiiooooooooo                  rrrrrrrrr"),
		ptr: 99,
	}, {
		input: []byte("" +
			"AGCTTTTCATTCTGACTGCAACGGGCAATATGTCTCTGTGTGGATTAAAAAAAGAGTCTCTGAC" +
			"AGCAGCTTCTGAACTGGTTACCTGCCGTGAGTAAATTAAAATTTTATTGACTTAGGTCACTAAA" +
			"TACTTTAACCAATATAGGCATAGCGCACAGACAGATAAAAATTACAGAGTACACAACATCCATG" +
			"AAACGCATTAGCACCACCATTACCACCACCATCACCACCACCATCACCATTACCATTACCACAG" +
			"GTAACGGTGCGGGCTGACGCGTACAGGAAACACAGAAAAAAGCCCGCACCTGACAGTGCGGGCT" +
			"TTTTTTTCGACCAAAGGTAACGAGGTAACAACCATGCGAGTGTTGAAGTTCGGCGGTACATCAG" +
			"TGGCAAATGCAGAACGTTTTCTGCGGGTTGCCGATATTCTGGAAAGCAATGCCAGGCAGGGGCA"),
		output: []byte("" +
			"TAGAATAAATGGAGACTCTAATACTCTACTGGAAACAGACCACAAACATACCTGGTCGTAGATT" +
			"CCCCCCATCCCTAAGAAACGAGTCCCCACATCATCACCTCGACTGGGCCGAGACTAAGCCCCCA" +
			"ACTGAACCCCCTTACGAAGGCGGAAGCTCCGCCCTGTAGAAAAGACGAATGCCAACCCCCGTAA" +
			"AAAAAAGAATAAAAGGCGAATAGCGCAATAGGGGAGCAATTTTCGTACTTATAGAGGAGTGATT" +
			"ATTCTTTCTAACACGGTGGACACTAGGCTATTTATTTGCGAAGATTTGGAACGGGCCCACAAAC" +
			"ACTGAGGGACGGATCGATATAGATGCTATCGGTGGGTGGTTTTATAATAAATAAGATATTGGTC" +
			"TTTCACTCCCCTGCAATCAGGCCGGCAGCGAATAAAAGACTTTGCATAGAGCTTTTACTGTTTC"),
		ptr: 99,
	}, {
		input:  testutil.MustLoadFile("testdata/gauntlet_test3.bin"),
		output: testutil.MustLoadFile("testdata/gauntlet_test3.bwt"),
		ptr:    0,
	}, {
		input:  testutil.MustLoadFile("testdata/silesia_ooffice.bin"),
		output: testutil.MustLoadFile("testdata/silesia_ooffice.bwt"),
		ptr:    461,
	}, {
		input:  testutil.MustLoadFile("testdata/silesia_xray.bin"),
		output: testutil.MustLoadFile("testdata/silesia_xray.bwt"),
		ptr:    1532,
	}, {
		input:  testutil.MustLoadFile("testdata/testfiles_test3.bin"),
		output: testutil.MustLoadFile("testdata/testfiles_test3.bwt"),
		ptr:    0,
	}, {
		input:  testutil.MustLoadFile("testdata/testfiles_test4.bin"),
		output: testutil.MustLoadFile("testdata/testfiles_test4.bwt"),
		ptr:    1026,
	}}

	bwt := new(burrowsWheelerTransform)
	for i, v := range vectors {
		output := append([]byte(nil), v.input...)
		ptr := bwt.Encode(output)
		input := append([]byte(nil), v.output...)
		bwt.Decode(input, ptr)

		if got, want, ok := testutil.BytesCompare(input, v.input); !ok {
			t.Errorf("test %d, input mismatch:\ngot  %s\nwant %s", i, got, want)
		}
		if got, want, ok := testutil.BytesCompare(output, v.output); !ok {
			t.Errorf("test %d, output mismatch:\ngot  %s\nwant %s", i, got, want)
		}
		if ptr != v.ptr {
			t.Errorf("test %d, pointer mismatch: got %d, want %d", i, ptr, v.ptr)
		}
	}
}
