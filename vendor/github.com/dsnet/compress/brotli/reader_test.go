// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"runtime"
	"testing"

	"github.com/dsnet/compress/internal/errors"
	"github.com/dsnet/compress/internal/testutil"
)

func TestReader(t *testing.T) {
	db := testutil.MustDecodeBitGen
	dh := testutil.MustDecodeHex
	lf := testutil.MustLoadFile

	errFuncs := map[string]func(error) bool{
		"IsUnexpectedEOF": func(err error) bool { return err == io.ErrUnexpectedEOF },
		"IsCorrupted":     errors.IsCorrupted,
	}
	vectors := []struct {
		desc   string // Description of the test
		input  []byte // Test input string
		output []byte // Expected output string
		inIdx  int64  // Expected input offset after reading
		outIdx int64  // Expected output offset after reading
		errf   string // Name of error checking callback
	}{{
		desc: "empty string (truncated)",
		errf: "IsUnexpectedEOF",
	}, {
		// Empty last block (WBITS: 16)
		desc:  "empty.00.br",
		input: dh("06"),
		inIdx: 1,
	}, {
		// Empty last block (WBITS: 17)
		desc:  "empty.01.br",
		input: dh("8101"),
		inIdx: 2,
	}, {
		desc:  "empty.02.br",
		input: dh("a101"),
		inIdx: 2,
	}, {
		desc:  "empty.03.br",
		input: dh("b101"),
		inIdx: 2,
	}, {
		// Empty last block (WBITS: 12)
		desc:  "empty.04.br",
		input: dh("c101"),
		inIdx: 2,
	}, {
		desc:  "empty.05.br",
		input: dh("d101"),
		inIdx: 2,
	}, {
		desc:  "empty.06.br",
		input: dh("e101"),
		inIdx: 2,
	}, {
		desc:  "empty.07.br",
		input: dh("f101"),
		inIdx: 2,
	}, {
		desc:  "empty.08.br",
		input: dh("33"),
		inIdx: 1,
	}, {
		desc:  "empty.09.br",
		input: dh("35"),
		inIdx: 1,
	}, {
		desc:  "empty.10.br",
		input: dh("37"),
		inIdx: 1,
	}, {
		// Empty last block (WBITS: 21)
		desc:  "empty.11.br",
		input: dh("39"),
		inIdx: 1,
	}, {
		desc:  "empty.12.br",
		input: dh("3b"),
		inIdx: 1,
	}, {
		desc:  "empty.13.br",
		input: dh("3d"),
		inIdx: 1,
	}, {
		desc:  "empty.14.br",
		input: dh("3f"),
		inIdx: 1,
	}, {
		desc:  "empty.15.br",
		input: dh("1a"),
		inIdx: 1,
	}, {
		desc:  "empty.16.br",
		input: dh("81160058"),
		inIdx: 4,
	}, {
		desc:  "empty.17.br",
		input: db("<<< X:0103 X:06*65535 X:03"),
		inIdx: 65538,
	}, {
		desc:  "empty.18.br",
		input: db("<<< X:010b00 X:581600*65535 X:5803"),
		inIdx: 196610,
	}, {
		desc:  "empty last block (WBITS: invalid)",
		input: dh("9101"),
		inIdx: 1,
		errf:  "IsCorrupted",
	}, {
		desc:  "empty last block (trash at the end)",
		input: dh("06ff"),
		inIdx: 1,
	}, {
		desc:  "empty last block (padding is non-zero)",
		input: dh("16"),
		inIdx: 1,
		errf:  "IsCorrupted",
	}, {
		desc:  "empty metadata block (MLEN: 0)",
		input: dh("0c03"),
		inIdx: 2,
	}, {
		desc:  "metadata block",
		input: dh("2c0648656c6c6f2c20776f726c642103"),
		inIdx: 16,
	}, {
		desc:  "metadata block (truncated)",
		input: dh("2c06"),
		inIdx: 2,
		errf:  "IsUnexpectedEOF",
	}, {
		desc:  "metadata block (use reserved bit)",
		input: dh("3c0648656c6c6f2c20776f726c642103"),
		inIdx: 1,
		errf:  "IsCorrupted",
	}, {
		desc:  "metadata block (meta padding is non-zero)",
		input: dh("2c8648656c6c6f2c20776f726c642103"),
		inIdx: 2,
		errf:  "IsCorrupted",
	}, {
		desc:  "metadata block (non-minimal MLEN)",
		input: dh("4c060048656c6c6f2c20776f726c642103"),
		inIdx: 3,
		errf:  "IsCorrupted",
	}, {
		desc:  "metadata block (MLEN: 1<<0)",
		input: dh("2c00ff03"),
		inIdx: 4,
	}, {
		desc:  "metadata block (MLEN: 1<<24)",
		input: db("<<< X:ecffff7f X:f0*16777216 X:03"),
		inIdx: 5 + 1<<24,
	}, {
		desc:   "raw data block",
		input:  dh("c0001048656c6c6f2c20776f726c642103"),
		output: dh("48656c6c6f2c20776f726c6421"),
		inIdx:  17,
		outIdx: 13,
	}, {
		desc:  "raw data block (truncated)",
		input: dh("c00010"),
		inIdx: 3,
		errf:  "IsUnexpectedEOF",
	}, {
		desc:  "raw data block (raw padding is non-zero)",
		input: dh("c000f048656c6c6f2c20776f726c642103"),
		inIdx: 3,
		errf:  "IsCorrupted",
	}, {
		desc:  "raw data block (non-minimal MLEN)",
		input: dh("c400000148656c6c6f2c20776f726c642103"),
		inIdx: 3,
		errf:  "IsCorrupted",
	}, {
		desc:   "raw data block (MLEN: 1<<0)",
		input:  dh("0000106103"),
		output: dh("61"),
		inIdx:  4 + 1<<0,
		outIdx: 1 << 0,
	}, {
		desc:   "raw data block (MLEN: 1<<24)",
		input:  db("<<< X:f8ffff1f X:f0*16777216 X:03"),
		output: db("<<< X:f0*16777216"),
		inIdx:  5 + 1<<24,
		outIdx: 1 << 24,
	}, {
		desc:   "simple prefix (|L|:1 |I|:1 |D|:1 MLEN:1)",
		input:  dh("00000000c4682010c0"),
		output: dh("a3"),
		inIdx:  9,
		outIdx: 1,
	}, {
		desc:   "simple prefix, out-of-order (|L|:2 |I|:1 |D|:1 MLEN:1)",
		input:  dh("00000000d4a8682010c001"),
		output: dh("a3"),
		inIdx:  11,
		outIdx: 1,
	}, {
		desc:   "simple prefix, non-unique (|L|:2 |I|:1 |D|:1 MLEN:1)",
		input:  dh("00000000d4e8682010c001"),
		output: dh(""),
		inIdx:  7,
		outIdx: 0,
		errf:   "IsCorrupted",
	}, {
		desc:   "simple prefix, out-of-order (|L|:3 |I|:1 |D|:1 MLEN:1)",
		input:  dh("0000000024e8e96820104003"),
		output: dh("a3"),
		inIdx:  12,
		outIdx: 1,
	}, {
		desc:   "simple prefix, out-of-order, no-tree-select (|L|:4 |I|:1 |D|:1 MLEN:1)",
		input:  dh("0000000034e8e968a840208006"),
		output: dh("a3"),
		inIdx:  13,
		outIdx: 1,
	}, {
		desc:   "simple prefix, out-of-order, yes-tree-select (|L|:4 |I|:1 |D|:1 MLEN:1)",
		input:  dh("0000000034e8e968e94020800d"),
		output: dh("a3"),
		inIdx:  13,
		outIdx: 1,
	}, {
		desc:   "simple prefix, max-sym-ok (|L|:1 |I|:2 |D|:1 MLEN:1)",
		input:  dh("00000000c46821f06b0006"),
		output: dh("a3"),
		inIdx:  11,
		outIdx: 1,
	}, {
		desc:   "simple prefix, max-sym-bad (|L|:1 |I|:2 |D|:1 MLEN:1)",
		input:  dh("00000000c46821006c0006"),
		output: dh(""),
		inIdx:  9,
		outIdx: 0,
		errf:   "IsCorrupted",
	}, {
		desc:   "complex prefix, skip-zero, terminate-clens-codes (|L|:1 |I|:2 |D|:1 MLEN:1)",
		input:  dh("0000000070472010c001"),
		output: dh("01"),
		inIdx:  10,
		outIdx: 1,
	}, {
		desc:   "complex prefix, skip-zero, terminate-clens-codes (|L|:1 |I|:2 |D|:1 MLEN:1)",
		input:  dh("0000000070c01d080470"),
		output: dh("01"),
		inIdx:  10,
		outIdx: 1,
	}, {
		desc:   "complex prefix, skip-zero, terminate-clens-codes (|L|:1 |I|:2 |D|:1 MLEN:2)",
		input:  dh("1000000070c01d1004d0"),
		output: dh("0100"),
		inIdx:  10,
		outIdx: 2,
	}, {
		desc:   "complex prefix, skip-zero, terminate-codes (|L|:1 |I|:4 |D|:1 MLEN:3)",
		input:  dh("20000000b0c100000056151804700e"),
		output: dh("030201"),
		inIdx:  15,
		outIdx: 3,
	}, {
		desc:   "complex prefix, skip-zero, under-subscribed (|L|:1 |I|:4 |D|:1 MLEN:3)",
		input:  dh("20000000b0c1000000ae2a3008e01c"),
		output: dh(""),
		inIdx:  10,
		outIdx: 0,
		errf:   "IsCorrupted",
	}, {
		desc:   "complex prefix, skip-zero, over-subscribed (|L|:1 |I|:4 |D|:1 MLEN:3)",
		input:  dh("20000000b0c1000000ac0a0c023807"),
		output: dh(""),
		inIdx:  10,
		outIdx: 0,
		errf:   "IsCorrupted",
	}, {
		desc:   "complex prefix, skip-zero, single clens (|L|:1 |I|:256 |D|:1 MLEN:4)",
		input:  dh("30000000000000020001420000a5ff5503"),
		output: dh("00a5ffaa"),
		inIdx:  17,
		outIdx: 4,
	}, {
		desc:   "complex prefix, skip-zero, single clens (|L|:1 |I|:32 |D|:1 MLEN:4)",
		input:  dh("3000000000c001000004080100faf7"),
		output: dh("00051f1b"),
		inIdx:  15,
		outIdx: 4,
	}, {
		desc:   "complex prefix, skip-zero, single clens, zero clen (|L|:1 |I|:? |D|:1 MLEN:4)",
		input:  dh("30000000007000000004080100faf7"),
		output: dh(""),
		inIdx:  10,
		outIdx: 0,
		errf:   "IsCorrupted",
	}, {
		desc:   "complex prefix, skip-zero, empty clens (|L|:1 |I|:? |D|:1 MLEN:4)",
		input:  dh("30000000000000000001420080fe3d"),
		output: dh(""),
		inIdx:  9,
		outIdx: 0,
		errf:   "IsCorrupted",
	}, {
		desc:   "complex prefix, skip-zero, single clens, rep-last clen (|L|:1 |I|:256 |D|:1 MLEN:4)",
		input:  dh("3000000000002000006a014200aa33cc5503"),
		output: dh("55cc33aa"),
		inIdx:  18,
		outIdx: 4,
	}, {
		desc:   "complex prefix, skip-zero, single clens, rep-last clen, over-subscribed (|L|:1 |I|:257 |D|:1 MLEN:4)",
		input:  dh("300000000000200000aa014200aa33cc5503"),
		output: dh(""),
		inIdx:  10,
		outIdx: 0,
		errf:   "IsCorrupted",
	}, {
		desc:   "complex prefix, skip-zero, single clens, rep-last clen, integer overflow (|L|:1 |I|:1018 |D|:1 MLEN:4)",
		input:  dh("3000000000002000002a070801a8ce30570d"),
		output: dh(""),
		inIdx:  11,
		outIdx: 0,
		errf:   "IsCorrupted",
	}, {
		desc:   "complex prefix, skip-two, single clens, rep-last clen (|L|:1 |I|:256 |D|:1 MLEN:4)",
		input:  dh("3000000008000f00805a801080ea0c73d5"),
		output: dh("55cc33aa"),
		inIdx:  17,
		outIdx: 4,
	}, {
		desc:   "complex prefix, skip-three, single clens, rep-last clen (|L|:1 |I|:256 |D|:1 MLEN:4)",
		input:  dh("300000000cc00300a0162004a03ac35c35"),
		output: dh("55cc33aa"),
		inIdx:  17,
		outIdx: 4,
	}, {
		desc: "complex prefix, skip-zero, linear clens (|L|:1 |I|:16 |D|:1 MLEN:16)",
		input: dh("f000000050555555ffff8bd5169058d43cb2fadcf77f201480dabdeff7f7efbf" +
			"fffddffffbfffe7fffff01"),
		output: dh("6162636465666768696a6b6c6d6e6f70"),
		inIdx:  43,
		outIdx: 16,
	}, {
		desc: "complex prefix, skip-zero, mixed clens (|L|:1 |I|:192 |D|:1 MLEN:16)",
		input: dh("f000000050555555ffffe37a310f369a4d4b80756cc779b0619a02a1002c29ab" +
			"ec066084eee99dfd67d8ac18"),
		output: dh("000240525356575e717a8abcbdbed7d9"),
		inIdx:  44,
		outIdx: 16,
	}, {
		desc:   "compressed string: \"Hello, world! Hello, world!\"",
		input:  dh("1b1a00008c946ed6540dc2825426d942de6a9668ea996c961e00"),
		output: dh("48656c6c6f2c20776f726c64212048656c6c6f2c20776f726c6421"),
		inIdx:  26,
		outIdx: 27,
	}, {
		desc:   "compressed string (padding is non-zero): \"Hello, world! Hello, world!\"",
		input:  dh("1b1a00008c946ed6540dc2825426d942de6a9668ea996c961e80"),
		output: dh("48656c6c6f2c20776f726c64212048656c6c6f2c20776f726c6421"),
		inIdx:  26,
		outIdx: 27,
		errf:   "IsCorrupted",
	}, {
		desc:   "x.br",
		input:  dh("0b00805803"),
		output: db(`<<< "X"`),
		inIdx:  5,
		outIdx: 1,
	}, {
		desc:   "x.00.br",
		input:  dh("0000105803"),
		output: db(`<<< "X"`),
		inIdx:  5,
		outIdx: 1,
	}, {
		desc:   "x.01.br",
		input:  dh("2c00580000085803"),
		output: db(`<<< "X"`),
		inIdx:  8,
		outIdx: 1,
	}, {
		desc:   "x.02.br",
		input:  dh("000010580d"),
		output: db(`<<< "X"`),
		inIdx:  5,
		outIdx: 1,
	}, {
		desc:   "x.03.br",
		input:  dh("a1000000008115080400"),
		output: db(`<<< "X"`),
		inIdx:  10,
		outIdx: 1,
	}, {
		desc:   "zeros.br",
		input:  dh("5bffff036002201e0b28f77e00"),
		output: db("<<< X:00*262144"),
		inIdx:  13,
		outIdx: 262144,
	}, {
		desc:   "xyzzy.br",
		input:  dh("0b028058797a7a7903"),
		output: db(`<<< "Xyzzy"`),
		inIdx:  9,
		outIdx: 5,
	}, {
		desc:   "10x10y.br",
		input:  dh("1b130000a4b0b2ea8147028a"),
		output: db(`<<< "X"*10 "Y"*10`),
		inIdx:  12,
		outIdx: 20,
	}, {
		desc:   "64x.br",
		input:  dh("1b3f000024b0e2998012"),
		output: db(`<<< "X"*64`),
		inIdx:  10,
		outIdx: 64,
	}, {
		desc:   "backward65536.br",
		input:  dh("5bff0001400a00ab167bac00484e73ed019203"),
		output: db(`<<< X:00*256 "X"*65280 X:00*256`),
		inIdx:  19,
		outIdx: 65792,
	}, {
		desc:   "quickfox.br",
		input:  dh("0b158054686520717569636b2062726f776e20666f78206a756d7073206f76657220746865206c617a7920646f6703"),
		output: db(`<<< "The quick brown fox jumps over the lazy dog"`),
		inIdx:  47,
		outIdx: 43,
	}, {
		desc:   "quickfox_repeated.br",
		input:  dh("5bffaf02c022795cfb5a8c423bf42555195a9299b135c8199e9e0a7b4b90b93c98c80940f3e6d94de46d651b2787135fa6e930967b3c15d8531c"),
		output: db(`<<< "The quick brown fox jumps over the lazy dog"*4096`),
		inIdx:  58,
		outIdx: 176128,
	}, {
		desc:   "ukkonooa.br",
		input:  lf("testdata/ukkonooa.br"),
		output: lf("testdata/ukkonooa"),
		inIdx:  69,
		outIdx: 119,
	}, {
		desc:   "monkey.br",
		input:  lf("testdata/monkey.br"),
		output: lf("testdata/monkey"),
		inIdx:  425,
		outIdx: 843,
	}, {
		desc:   "random_org_10k.bin.br",
		input:  lf("testdata/random_org_10k.bin.br"),
		output: lf("testdata/random_org_10k.bin"),
		inIdx:  10004,
		outIdx: 10000,
	}, {
		desc:   "asyoulik.txt.br",
		input:  lf("testdata/asyoulik.txt.br"),
		output: lf("testdata/asyoulik.txt"),
		inIdx:  45687,
		outIdx: 125179,
	}, {
		desc:   "compressed_file.br",
		input:  lf("testdata/compressed_file.br"),
		output: lf("testdata/compressed_file"),
		inIdx:  50100,
		outIdx: 50096,
	}, {
		desc:   "compressed_repeated.br",
		input:  lf("testdata/compressed_repeated.br"),
		output: lf("testdata/compressed_repeated"),
		inIdx:  50299,
		outIdx: 144224,
	}, {
		desc:   "alice29.txt.br",
		input:  lf("testdata/alice29.txt.br"),
		output: lf("testdata/alice29.txt"),
		inIdx:  50096,
		outIdx: 152089,
	}, {
		desc:   "lcet10.txt.br",
		input:  lf("testdata/lcet10.txt.br"),
		output: lf("testdata/lcet10.txt"),
		inIdx:  124719,
		outIdx: 426754,
	}, {
		desc:   "mapsdatazrh.br",
		input:  lf("testdata/mapsdatazrh.br"),
		output: lf("testdata/mapsdatazrh"),
		inIdx:  161743,
		outIdx: 285886,
	}, {
		desc:   "plrabn12.txt.br",
		input:  lf("testdata/plrabn12.txt.br"),
		output: lf("testdata/plrabn12.txt"),
		inIdx:  174771,
		outIdx: 481861,
	}}

	for i, v := range vectors {
		rd, err := NewReader(bytes.NewReader(v.input), nil)
		if err != nil {
			t.Errorf("test %d, unexpected NewReader error: %v", i, err)
		}
		output, err := ioutil.ReadAll(rd)
		if cerr := rd.Close(); cerr != nil {
			err = cerr
		}

		if got, want, ok := testutil.BytesCompare(output, v.output); !ok {
			t.Errorf("test %d, %s\noutput mismatch:\ngot  %s\nwant %s", i, v.desc, got, want)
		}
		if rd.InputOffset != v.inIdx {
			t.Errorf("test %d, %s\ninput offset mismatch: got %d, want %d", i, v.desc, rd.InputOffset, v.inIdx)
		}
		if rd.OutputOffset != v.outIdx {
			t.Errorf("test %d, %s\noutput offset mismatch: got %d, want %d", i, v.desc, rd.OutputOffset, v.outIdx)
		}
		if v.errf != "" && !errFuncs[v.errf](err) {
			t.Errorf("test %d, mismatching error:\ngot %v\nwant %s(err) == true", i, err, v.errf)
		} else if v.errf == "" && err != nil {
			t.Errorf("test %d, unexpected error: got %v", i, err)
		}

		if *zcheck {
			output, err := cmdDecompress(v.input)
			if got, want := bool(v.errf == ""), bool(err == nil); got != want {
				t.Errorf("test %d, pass mismatch: got %v, want %v", i, got, err)
			}
			if got, want, ok := testutil.BytesCompare(output, v.output); !ok && err == nil {
				t.Errorf("test %d, output mismatch:\ngot  %s\nwant %s", i, got, want)
			}
		}
	}
}

func benchmarkDecode(b *testing.B, testfile string) {
	b.StopTimer()
	b.ReportAllocs()

	input, err := ioutil.ReadFile("testdata/" + testfile)
	if err != nil {
		b.Fatal(err)
	}
	rd, err := NewReader(bytes.NewReader(input), nil)
	if err != nil {
		b.Fatal(err)
	}
	output, err := ioutil.ReadAll(rd)
	if err != nil {
		b.Fatal(err)
	}

	nb := int64(len(output))
	output = nil
	runtime.GC()

	b.SetBytes(nb)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		rd, err := NewReader(bufio.NewReader(bytes.NewReader(input)), nil)
		if err != nil {
			b.Fatalf("unexpected NewReader error: %v", err)
		}
		cnt, err := io.Copy(ioutil.Discard, rd)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
		if cnt != nb {
			b.Fatalf("unexpected count: got %d, want %d", cnt, nb)
		}
	}
}

func BenchmarkDecodeDigitsSpeed1e4(b *testing.B)    { benchmarkDecode(b, "digits-speed-1e4.br") }
func BenchmarkDecodeDigitsSpeed1e5(b *testing.B)    { benchmarkDecode(b, "digits-speed-1e5.br") }
func BenchmarkDecodeDigitsSpeed1e6(b *testing.B)    { benchmarkDecode(b, "digits-speed-1e6.br") }
func BenchmarkDecodeDigitsDefault1e4(b *testing.B)  { benchmarkDecode(b, "digits-default-1e4.br") }
func BenchmarkDecodeDigitsDefault1e5(b *testing.B)  { benchmarkDecode(b, "digits-default-1e5.br") }
func BenchmarkDecodeDigitsDefault1e6(b *testing.B)  { benchmarkDecode(b, "digits-default-1e6.br") }
func BenchmarkDecodeDigitsCompress1e4(b *testing.B) { benchmarkDecode(b, "digits-best-1e4.br") }
func BenchmarkDecodeDigitsCompress1e5(b *testing.B) { benchmarkDecode(b, "digits-best-1e5.br") }
func BenchmarkDecodeDigitsCompress1e6(b *testing.B) { benchmarkDecode(b, "digits-best-1e6.br") }
func BenchmarkDecodeTwainSpeed1e4(b *testing.B)     { benchmarkDecode(b, "twain-speed-1e4.br") }
func BenchmarkDecodeTwainSpeed1e5(b *testing.B)     { benchmarkDecode(b, "twain-speed-1e5.br") }
func BenchmarkDecodeTwainSpeed1e6(b *testing.B)     { benchmarkDecode(b, "twain-speed-1e6.br") }
func BenchmarkDecodeTwainDefault1e4(b *testing.B)   { benchmarkDecode(b, "twain-default-1e4.br") }
func BenchmarkDecodeTwainDefault1e5(b *testing.B)   { benchmarkDecode(b, "twain-default-1e5.br") }
func BenchmarkDecodeTwainDefault1e6(b *testing.B)   { benchmarkDecode(b, "twain-default-1e6.br") }
func BenchmarkDecodeTwainCompress1e4(b *testing.B)  { benchmarkDecode(b, "twain-best-1e4.br") }
func BenchmarkDecodeTwainCompress1e5(b *testing.B)  { benchmarkDecode(b, "twain-best-1e5.br") }
func BenchmarkDecodeTwainCompress1e6(b *testing.B)  { benchmarkDecode(b, "twain-best-1e6.br") }
