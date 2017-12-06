// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package prefix

import (
	"bufio"
	"bytes"
	"io"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/dsnet/compress"
	"github.com/dsnet/compress/internal"
	"github.com/dsnet/compress/internal/testutil"
)

const testSize = 1000

var (
	testVector = testutil.MustDecodeHex("" +
		"f795bd4a52e29ed713d3ff6eef91fc7f0735428e2bff9ee8e85f00bfa9c4aa3a" +
		"beab8e3d59248dd6b5ff1e3cc1f49f024bff6edffd3f04ea797b45cc128eabe9" +
		"9eb55197a2ff9ea118f2bf0c076ab6e095ffaea789ff063e7ff816d299c31411" +
		"90baffeef5ff355b79ff9ed1d7d3fe8f01ff1e5a56f32f02b9bef6ae3932ff2e" +
		"eeff057637f086ebbbff9e62f96915af14ffabcb1381f024a50bf86425ffee80" +
		"7bffeefadf0327534c04b82835a07e09821915ce8cff6ea4dfe60de7f29e7b0f" +
		"16ffae5f8190ff16ff9e64f81f0285ffee0daa7f04dd383667c3ff1eebffff4f" +
		"2cc3553c4236ff8efd6b57e7c69932351ebf858722b2ffae2399d913ffce43ff" +
		"29e86bc831330a66ff1e982fa98286ccbc29c6d03429b492349bff1e83ff4700" +
		"37ec814173da6a0c79ffee99ff47e0e632e85ccdc44f21ff6eedbf0094e11b07" +
		"99a0cfe5b789ee58ff1e462bfe75eb1aae7754bc62b37f0e699094ff9e95ff5f" +
		"0461b381c3557076df1f4a6d2ff3faff9ef9af0246f6e9abf9e4cd0be7ff9ee9" +
		"90eeff1bd24a0d7b67ffee2dff5b49b8b0828b0d51669fcb04ff2e2cbbd30181" +
		"c06c1d39137a4bff97a92cff4e1ffe639c0589759657c71a0bceff1ed83f0662" +
		"6f6eea066aff6e1476483462a2c0a213ffaefa7f00faecc24e592e9eff6e72da" +
		"18ff23f9a598c26d479f52a6ce97e7891556ff1efcbf05422263c89384f18b25" +
		"f1c5031c78ff9e9dcbc4178f639e298a0e8e8a40ff2e0042300b76b9ff6e2af5" +
		"fadf023288d043fcff1e0ffe39ff9e75ff7ee8c8a68dd063e4ed7d5b7fbbe8c3" +
		"3aff1e66f18f03ceb43fb0a49fcb78cb086b107cf8d3ffae6981bdff1f744f57" +
		"a3d990881b50c3ffee74bb2ffee89d3ef0419f278eff1e22710f3ed9ff9e3b33" +
		"f98f02c5b575cfb5ecd408ffeefed701ff9eb7019a7f2fd610c4670d667dc0f5" +
		"bbff6ed0578cff2200bffbd22521f5c59430ff6ef63f0c1c5f64bce753f261ff" +
		"2d2e9de9b4feff1eefc0fe9b8dc0d5d91cff1e8b84c9ffb701883442f5ff1e18" +
		"0aff7da3d0c04abe9b9fbda906f55fe5f6ceff4e21fa0f000185990b348ba2d5" +
		"a88903207be2ffeefe6f0287bcfbffee5abcf39f030a95b5d6ff9e8bcb7f0ef0" +
		"a26bb9fa463591589de79bd633ffaecf7f122fb470144fff9e08ecc57f16f2a3" +
		"6ab646e6f883b0d46b2e7eafadff1ea1ff5f07be86ff1e8a61f504b469527523" +
		"85339cff9e90e2f316549a364bf62469d053ad0b1a552c1affeecb27f13f0875" +
		"5a008a5f064098afff1efe2f01ce0808826b4d5f1fa3d71c1726ff6e9064f13f" +
		"01068c5269fb7a99e55c7869ff9e22f8bf02f4cb0d5e3ae00d3dc3e1ff9eddbd" +
		"520ffac3f8a16bb0b1e7f5d050e0ffee950d10ff238fb3daade442040dd0e8cf" +
		"f50cffeed2fa5f008ebbd4dacbeb8fcd7c5ba440bf",
	)

	testCodes = func() (codes PrefixCodes) {
		for i := 0; i < 100; i++ {
			codes = append(codes, PrefixCode{Sym: uint32(len(codes)), Cnt: 0})
		}
		for i := 0; i < 25; i++ {
			codes = append(codes, PrefixCode{Sym: uint32(len(codes)), Cnt: 10})
		}
		for i := 0; i < 5; i++ {
			codes = append(codes, PrefixCode{Sym: uint32(len(codes)), Cnt: 1000})
		}
		codes.SortByCount()
		if err := GenerateLengths(codes, 15); err != nil {
			panic(err)
		}
		codes.SortBySymbol()
		if err := GeneratePrefixes(codes); err != nil {
			panic(err)
		}
		return codes
	}()

	testRanges = MakeRangeCodes(0, []uint{0, 1, 2, 3, 4})
)

func TestReader(t *testing.T) {
	readers := map[string]func([]byte) io.Reader{
		"io.Reader": func(b []byte) io.Reader {
			return struct{ io.Reader }{bytes.NewReader(b)}
		},
		"bytes.Buffer": func(b []byte) io.Reader {
			return bytes.NewBuffer(b)
		},
		"bytes.Reader": func(b []byte) io.Reader {
			return bytes.NewReader(b)
		},
		"string.Reader": func(b []byte) io.Reader {
			return strings.NewReader(string(b))
		},
		"compress.ByteReader": func(b []byte) io.Reader {
			return struct{ compress.ByteReader }{bytes.NewReader(b)}
		},
		"compress.BufferedReader": func(b []byte) io.Reader {
			return struct{ compress.BufferedReader }{bufio.NewReader(bytes.NewReader(b))}
		},
	}
	endians := map[string]bool{"littleEndian": false, "bigEndian": true}

	var i int
	for ne, endian := range endians {
		for nr, newReader := range readers {
			var br Reader
			buf := make([]byte, len(testVector))
			copy(buf, testVector)
			if endian {
				for i, c := range buf {
					buf[i] = internal.ReverseLUT[c]
				}
			}
			rd := newReader(buf)
			br.Init(rd, endian)

			var pd Decoder
			pd.Init(testCodes)

			r := testutil.NewRand(0)
		loop:
			for j := 0; br.BitsRead() < 8*testSize; j++ {
				switch j % 4 {
				case 0:
					// Test unaligned Read.
					if br.numBits%8 != 0 {
						cnt, err := br.Read([]byte{0})
						if cnt != 0 {
							t.Errorf("test %d, %s %s, write count mismatch: got %d, want 0", i, ne, nr, cnt)
							break loop
						}
						if err == nil {
							t.Errorf("test %d, %s %s, unexpected write success", i, ne, nr)
							break loop
						}
					}

					pads := br.ReadPads()
					if pads != 0 {
						t.Errorf("test %d, %s %s, bit padding mismatch: got %d, want 0", i, ne, nr, pads)
						break loop
					}
					want := r.Bytes(r.Intn(16))
					if endian {
						for i, c := range want {
							want[i] = internal.ReverseLUT[c]
						}
					}
					got := make([]byte, len(want))
					cnt, err := io.ReadFull(&br, got)
					if cnt != len(want) {
						t.Errorf("test %d, %s %s, read count mismatch: got %d, want %d", i, ne, nr, cnt, len(want))
						break loop
					}
					if err != nil {
						t.Errorf("test %d, %s %s, unexpected read error: got %v", i, ne, nr, err)
						break loop
					}
					if bytes.Compare(want, got) != 0 {
						t.Errorf("test %d, %s %s, read bytes mismatch:\ngot  %x\nwant %x", i, ne, nr, got, want)
						break loop
					}
				case 1:
					n := int(testRanges.End() - testRanges.Base())
					want := uint(testRanges.Base() + uint32(r.Intn(n)))
					got := br.ReadOffset(&pd, testRanges)
					if got != want {
						t.Errorf("test %d, %s %s, read offset mismatch: got %d, want %d", i, ne, nr, got, want)
						break loop
					}
				case 2:
					nb := uint(r.Intn(24))
					want := uint(r.Int() & (1<<nb - 1))
					got, ok := br.TryReadBits(nb)
					if !ok {
						got = br.ReadBits(nb)
					}
					if got != want {
						t.Errorf("test %d, %s %s, read bits mismatch: got %d, want %d", i, ne, nr, got, want)
						break loop
					}
				case 3:
					want := uint(testCodes[r.Intn(len(testCodes))].Sym)
					got, ok := br.TryReadSymbol(&pd)
					if !ok {
						got = br.ReadSymbol(&pd)
					}
					if got != want {
						t.Errorf("test %d, %s %s, read symbol mismatch: got %d, want %d", i, ne, nr, got, want)
						break loop
					}
				}
			}

			pads := br.ReadPads()
			if pads != 0 {
				t.Errorf("test %d, %s %s, bit padding mismatch: got %d, want 0", i, ne, nr, pads)
			}
			ofs, err := br.Flush()
			if br.numBits != 0 {
				t.Errorf("test %d, %s, bit buffer not drained: got %d, want < 8", i, ne, br.numBits)
			}
			if ofs != int64(len(testVector)) {
				t.Errorf("test %d, %s, offset mismatch: got %d, want %d", i, ne, ofs, len(testVector))
			}
			if err != nil {
				t.Errorf("test %d, %s, unexpected flush error: got %v", i, ne, err)
			}
			i++
		}
	}
}

func TestWriter(t *testing.T) {
	endians := map[string]bool{"littleEndian": false, "bigEndian": true}

	var i int
	for ne, endian := range endians {
		var bw Writer
		wr := bytes.NewBuffer(nil)
		bw.Init(wr, endian)

		var pe Encoder
		pe.Init(testCodes)

		var re RangeEncoder
		re.Init(testRanges)

		r := testutil.NewRand(0)
	loop:
		for j := 0; bw.BitsWritten() < 8*testSize; j++ {
			switch j % 4 {
			case 0:
				// Test unaligned Write.
				if bw.numBits%8 != 0 {
					cnt, err := bw.Write([]byte{0})
					if cnt != 0 {
						t.Errorf("test %d, %s, write count mismatch: got %d, want 0", i, ne, cnt)
						break loop
					}
					if err == nil {
						t.Errorf("test %d, %s, unexpected write success", i, ne)
						break loop
					}
				}

				bw.WritePads(0)
				b := r.Bytes(r.Intn(16))
				if endian {
					for i, c := range b {
						b[i] = internal.ReverseLUT[c]
					}
				}
				cnt, err := bw.Write(b)
				if cnt != len(b) {
					t.Errorf("test %d, %s, write count mismatch: got %d, want %d", i, ne, cnt, len(b))
					break loop
				}
				if err != nil {
					t.Errorf("test %d, %s, unexpected write error: got %v", i, ne, err)
					break loop
				}
			case 1:
				n := int(testRanges.End() - testRanges.Base())
				ofs := uint(testRanges.Base() + uint32(r.Intn(n)))
				bw.WriteOffset(ofs, &pe, &re)
			case 2:
				nb := uint(r.Intn(24))
				val := uint(r.Int() & (1<<nb - 1))
				ok := bw.TryWriteBits(val, nb)
				if !ok {
					bw.WriteBits(val, nb)
				}
			case 3:
				sym := uint(testCodes[r.Intn(len(testCodes))].Sym)
				ok := bw.TryWriteSymbol(sym, &pe)
				if !ok {
					bw.WriteSymbol(sym, &pe)
				}
			}
		}

		// Flush the Writer.
		bw.WritePads(0)
		ofs, err := bw.Flush()
		if bw.numBits != 0 {
			t.Errorf("test %d, %s, bit buffer not drained: got %d, want 0", i, ne, bw.numBits)
		}
		if bw.cntBuf != 0 {
			t.Errorf("test %d, %s, byte buffer not drained: got %d, want 0", i, ne, bw.cntBuf)
		}
		if ofs != int64(wr.Len()) {
			t.Errorf("test %d, %s, offset mismatch: got %d, want %d", i, ne, ofs, wr.Len())
		}
		if err != nil {
			t.Errorf("test %d, %s, unexpected flush error: got %v", i, ne, err)
		}

		// Check that output matches expected.
		buf := wr.Bytes()
		if endian {
			for i, c := range buf {
				buf[i] = internal.ReverseLUT[c]
			}
		}
		if bytes.Compare(buf, testVector) != 0 {
			t.Errorf("test %d, %s, output string mismatch:\ngot  %x\nwant %x", i, ne, buf, testVector)
		}
		i++
	}
}

func TestGenerate(t *testing.T) {
	r := testutil.NewRand(0)
	makeCodes := func(freqs []uint) PrefixCodes {
		codes := make(PrefixCodes, len(freqs))
		for i, j := range r.Perm(len(freqs)) {
			codes[i] = PrefixCode{Sym: uint32(i), Cnt: uint32(freqs[j])}
		}
		codes.SortByCount()
		return codes
	}

	vectors := []struct {
		maxBits uint // Maximum prefix bit-length (0 to skip GenerateLengths)
		input   PrefixCodes
		valid   bool
	}{{
		maxBits: 15,
		input:   makeCodes([]uint{}),
		valid:   true,
	}, {
		maxBits: 15,
		input:   makeCodes([]uint{0}),
		valid:   true,
	}, {
		maxBits: 15,
		input:   makeCodes([]uint{5}),
		valid:   true,
	}, {
		maxBits: 15,
		input:   makeCodes([]uint{0, 0}),
		valid:   true,
	}, {
		maxBits: 15,
		input:   makeCodes([]uint{5, 15}),
		valid:   true,
	}, {
		maxBits: 15,
		input:   makeCodes([]uint{1, 1, 2, 4}),
		valid:   true,
	}, {
		maxBits: 2,
		input:   makeCodes([]uint{1, 1, 2, 4}),
		valid:   true,
	}, {
		maxBits: 7,
		input:   makeCodes([]uint{100, 101, 102, 103}),
		valid:   true,
	}, {
		maxBits: 10,
		input:   makeCodes([]uint{2, 2, 2, 2, 5, 5, 5}),
		valid:   true,
	}, {
		maxBits: 15,
		input:   makeCodes([]uint{1, 2, 3, 4, 5, 6, 7, 8, 9}),
		valid:   true,
	}, {
		maxBits: 15,
		input:   makeCodes([]uint{0, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9}),
		valid:   true,
	}, {
		maxBits: 7,
		input:   makeCodes([]uint{0, 0, 2, 3, 4, 4, 4, 5, 5, 6, 6, 7, 7, 9, 10, 11, 13, 15}),
		valid:   true,
	}, {
		maxBits: 20,
		input:   makeCodes([]uint{1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536}),
		valid:   true,
	}, {
		maxBits: 12,
		input:   makeCodes([]uint{1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536}),
		valid:   true,
	}, {
		maxBits: 15,
		input: makeCodes([]uint{
			1, 1, 1, 1, 1, 2, 2, 3, 3, 4, 4, 4, 4, 6, 6, 7, 7, 8, 8, 9, 9, 11, 11,
			11, 11, 14, 15, 15, 17, 17, 18, 19, 19, 19, 20, 20, 21, 24, 26, 26, 31,
			32, 34, 35, 38, 40, 43, 47, 48, 50, 59, 62, 63, 75, 78, 79, 85, 86, 97,
			100, 100, 102, 114, 119, 128, 128, 139, 153, 166, 170, 174, 182, 184,
			185, 186, 205, 325, 536, 948, 1610, 2555, 2628, 3741,
		}),
		valid: true,
	}, {
		// Input counts are not sorted in ascending order.
		maxBits: 15,
		input: []PrefixCode{
			{Sym: 0, Cnt: 3},
			{Sym: 1, Cnt: 2},
			{Sym: 2, Cnt: 1},
		},
		valid: false,
	}, {
		// Input symbols are not sorted in ascending order.
		input: []PrefixCode{
			{Sym: 2, Len: 1},
			{Sym: 1, Len: 2},
			{Sym: 0, Len: 2},
		},
		valid: false,
	}, {
		// Input symbols are not unique.
		input: []PrefixCode{
			{Sym: 5, Len: 1},
			{Sym: 5, Len: 1},
		},
		valid: false,
	}, {
		// Invalid small tree.
		input: []PrefixCode{
			{Sym: 0, Len: 500},
		},
		valid: false,
	}, {
		// Some bit-length is too short.
		input: []PrefixCode{
			{Sym: 0, Len: 1},
			{Sym: 1, Len: 2},
			{Sym: 2, Len: 0},
		},
		valid: false,
	}, {
		// Under-subscribed tree.
		input: []PrefixCode{
			{Sym: 0, Len: 3},
			{Sym: 1, Len: 4},
			{Sym: 2, Len: 3},
		},
		valid: false,
	}, {
		// Over-subscribed tree.
		input: []PrefixCode{
			{Sym: 0, Len: 1},
			{Sym: 1, Len: 3},
			{Sym: 2, Len: 4},
			{Sym: 3, Len: 3},
			{Sym: 4, Len: 2},
		},
		valid: false,
	}, {
		// Over-subscribed tree (golang.org/issues/5915).
		input: []PrefixCode{
			{Sym: 0, Len: 4},
			{Sym: 3, Len: 6},
			{Sym: 4, Len: 4},
			{Sym: 5, Len: 3},
			{Sym: 6, Len: 2},
			{Sym: 7, Len: 3},
			{Sym: 8, Len: 3},
			{Sym: 9, Len: 4},
			{Sym: 10, Len: 4},
			{Sym: 11, Len: 5},
			{Sym: 16, Len: 5},
			{Sym: 17, Len: 5},
			{Sym: 18, Len: 6},
			{Sym: 29, Len: 11},
			{Sym: 51, Len: 7},
			{Sym: 52, Len: 8},
			{Sym: 53, Len: 6},
			{Sym: 55, Len: 11},
			{Sym: 57, Len: 8},
			{Sym: 59, Len: 6},
			{Sym: 60, Len: 6},
			{Sym: 61, Len: 10},
			{Sym: 62, Len: 8},
		},
		valid: false,
	}, {
		// Over-subscribed tree (golang.org/issues/5962).
		input: []PrefixCode{
			{Sym: 0, Len: 4},
			{Sym: 3, Len: 6},
			{Sym: 4, Len: 4},
			{Sym: 5, Len: 3},
			{Sym: 6, Len: 2},
			{Sym: 7, Len: 3},
			{Sym: 8, Len: 3},
			{Sym: 9, Len: 4},
			{Sym: 10, Len: 4},
			{Sym: 11, Len: 5},
			{Sym: 16, Len: 5},
			{Sym: 17, Len: 5},
			{Sym: 18, Len: 6},
			{Sym: 29, Len: 11},
		},
		valid: false,
	}, {
		// Under-subscribed tree (golang.org/issues/6255).
		input: []PrefixCode{
			{Sym: 0, Len: 11},
			{Sym: 1, Len: 13},
		},
		valid: false,
	}}

	for i, v := range vectors {
		var sum uint32
		var maxLen uint
		var lens []int
		var symBits [valueBits + 1]uint

		codes := v.input
		if v.maxBits == 0 {
			goto genPrefixes
		}

		if err := GenerateLengths(codes, v.maxBits); err != nil {
			if v.valid {
				t.Errorf("test %d, unexpected failure", i)
			}
			continue
		}

		for _, c := range codes {
			if maxLen < uint(c.Len) {
				maxLen = uint(c.Len)
			}
			symBits[c.Len]++
			lens = append(lens, int(c.Len))
			sum += c.Cnt
		}

		if !codes.checkLengths() {
			t.Errorf("test %d, incomplete tree generated", i)
		}
		if !sort.IsSorted(sort.Reverse(sort.IntSlice(lens))) {
			t.Errorf("test %d, bit-lengths are not sorted:\ngot %v", i, lens)
		}
		if maxLen > v.maxBits {
			t.Errorf("test %d, max bit-length exceeded: %d not in 1..%d", i, maxLen, v.maxBits)
		}

		// The whole point of prefix encoding is that the resulting bit-lengths
		// produce an encoding with close to ideal entropy. Thus, compute the
		// best-case entropy and check that we're not too far from it.
		if len(codes) >= 4 && sum > 0 {
			var worst, got, best float64
			worst = math.Log2(float64(len(codes)))
			got = float64(codes.Length()) / float64(sum)
			for _, c := range codes {
				if c.Cnt > 0 {
					p := float64(c.Cnt) / float64(sum)
					best += -(p * math.Log2(p))
				}
			}

			if got > worst {
				t.Errorf("test %d, actual entropy worst than worst-case: %0.3f > %0.3f", i, got, worst)
			}
			if got < best {
				t.Errorf("test %d, actual entropy better than best-case: %0.3f < %0.3f", i, got, best)
			}
			if got > 1.15*best {
				t.Errorf("test %d, actual entropy too high: %0.3f > %0.3f", i, got, 1.15*best)
			}
		}
		codes.SortBySymbol()

	genPrefixes:
		if err := GeneratePrefixes(codes); err != nil {
			if v.valid {
				t.Errorf("test %d, unexpected failure", i)
			}
			continue
		}

		if !codes.checkPrefixes() {
			t.Errorf("test %d, tree with non-unique prefixes generated", i)
		}
		if !codes.checkCanonical() {
			t.Errorf("test %d, tree with non-canonical prefixes generated", i)
		}
		if !v.valid {
			t.Errorf("test %d, unexpected success", i)
		}
	}
}

func TestPrefix(t *testing.T) {
	makeCodes := func(freqs []uint) PrefixCodes {
		codes := make(PrefixCodes, len(freqs))
		for i, n := range freqs {
			codes[i] = PrefixCode{Sym: uint32(i), Cnt: uint32(n)}
		}
		codes.SortByCount()
		if err := GenerateLengths(codes, 15); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		codes.SortBySymbol()
		if err := GeneratePrefixes(codes); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return codes
	}

	vectors := []struct {
		codes PrefixCodes
	}{{
		codes: makeCodes([]uint{}),
	}, {
		codes: makeCodes([]uint{0}),
	}, {
		codes: makeCodes([]uint{2, 4, 3, 2, 2, 4}),
	}, {
		codes: makeCodes([]uint{2, 2, 2, 2, 5, 5, 5}),
	}, {
		codes: makeCodes([]uint{100, 101, 102, 103}),
	}, {
		codes: makeCodes([]uint{
			1, 1, 1, 1, 1, 2, 2, 2, 3, 4, 5, 6, 6, 7, 8, 9, 9, 10, 11, 11, 12, 12,
			14, 15, 15, 16, 18, 18, 19, 19, 20, 20, 20, 25, 25, 27, 29, 31, 32, 35,
			39, 44, 47, 52, 60, 62, 71, 73, 74, 82, 86, 97, 98, 103, 108, 110, 112,
			125, 130, 142, 154, 155, 160, 185, 198, 204, 204, 219, 222, 259, 262,
			292, 296, 302, 334, 434, 450, 679, 697, 1032, 1441, 1888, 1892, 2188,
		}),
	}, {
		codes: testCodes,
	}, {
		// Sparsely allocated symbols.
		codes: []PrefixCode{
			{Sym: 16, Val: 0, Len: 1},
			{Sym: 32, Val: 1, Len: 2},
			{Sym: 64, Val: 3, Len: 3},
			{Sym: 128, Val: 7, Len: 3},
		},
	}, {
		// Large number of symbols.
		codes: func() PrefixCodes {
			freqs := make([]uint, 4096)
			for i := range freqs {
				freqs[i] = uint(i)
			}
			return makeCodes(freqs)
		}(),
	}, {
		// Max RLE codes from Brotli.
		codes: func() (codes PrefixCodes) {
			codes = PrefixCodes{{Sym: 0, Val: 0, Len: 1}}
			for i := uint32(0); i < 16; i++ {
				code := PrefixCode{Sym: i + 1, Val: i<<1 | 1, Len: 5}
				codes = append(codes, code)
			}
			return codes
		}(),
	}, {
		// Window bits codes from Brotli.
		codes: func() (codes PrefixCodes) {
			for i := uint32(9); i <= 24; i++ {
				var code PrefixCode
				switch {
				case i == 16:
					code = PrefixCode{Sym: i, Val: (i-16)<<0 | 0, Len: 1} // Symbols: 16
				case i > 17:
					code = PrefixCode{Sym: i, Val: (i-17)<<1 | 1, Len: 4} // Symbols: 18..24
				case i < 17:
					code = PrefixCode{Sym: i, Val: (i-8)<<4 | 1, Len: 7} // Symbols: 9..15
				default:
					code = PrefixCode{Sym: i, Val: (i-17)<<4 | 1, Len: 7} // Symbols: 17
				}
				codes = append(codes, code)
			}
			codes[0].Sym = 0
			return codes
		}(),
	}, {
		// Count codes from Brotli.
		codes: func() (codes PrefixCodes) {
			codes = PrefixCodes{{Sym: 1, Val: 0, Len: 1}}
			c := codes[len(codes)-1]
			for i := uint32(0); i < 8; i++ {
				for j := uint32(0); j < 1<<i; j++ {
					c.Sym = c.Sym + 1
					c.Val = j<<4 | i<<1 | 1
					c.Len = uint32(i + 4)
					codes = append(codes, c)
				}
			}
			return codes
		}(),
	}, {
		// Fixed literal codes from DEFLATE.
		codes: func() (codes PrefixCodes) {
			for i := 0; i < 144; i++ {
				codes = append(codes, PrefixCode{Sym: uint32(i), Len: 8})
			}
			for i := 144; i < 256; i++ {
				codes = append(codes, PrefixCode{Sym: uint32(i), Len: 9})
			}
			for i := 256; i < 280; i++ {
				codes = append(codes, PrefixCode{Sym: uint32(i), Len: 7})
			}
			for i := 280; i < 288; i++ {
				codes = append(codes, PrefixCode{Sym: uint32(i), Len: 8})
			}
			if err := GeneratePrefixes(codes); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			return codes
		}(),
	}, {
		// Fixed distance codes from DEFLATE.
		codes: func() (codes PrefixCodes) {
			for i := 0; i < 32; i++ {
				codes = append(codes, PrefixCode{Sym: uint32(i), Len: 5})
			}
			if err := GeneratePrefixes(codes); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			return codes
		}(),
	}}

	for i, v := range vectors {
		// Generate an arbitrary prefix Decoder and Encoder.
		var pd Decoder
		var pe Encoder
		pd.Init(v.codes)
		pe.Init(v.codes)
		if len(v.codes) == 0 {
			continue
		}

		// Create an arbitrary list of symbols to encode.
		r := testutil.NewRand(0)
		syms := make([]uint, 1000)
		for i := range syms {
			syms[i] = uint(v.codes[r.Intn(len(v.codes))].Sym)
		}

		// Setup a Reader and Writer.
		var buf bytes.Buffer
		var rd Reader
		var wr Writer
		rdwr := struct {
			io.Reader
			io.ByteReader
			io.Writer
		}{&buf, &buf, &buf}
		rd.Init(rdwr, false)
		wr.Init(rdwr, false)

		// Write some symbols.
		for _, sym := range syms {
			ok := wr.TryWriteSymbol(sym, &pe)
			if !ok {
				wr.WriteSymbol(sym, &pe)
			}
		}
		wr.WritePads(0)
		if _, err := wr.Flush(); err != nil {
			t.Errorf("test %d, unexpected Writer error: %v", i, err)
		}

		// Verify some Writer statistics.
		if wr.Offset != int64(buf.Len()) {
			t.Errorf("test %d, offset mismatch: got %d, want %d", i, wr.Offset, buf.Len())
		}
		if wr.numBits != 0 {
			t.Errorf("test %d, residual bits remaining: got %d, want 0", i, wr.numBits)
		}
		if wr.cntBuf != 0 {
			t.Errorf("test %d, residual bytes remaining: got %d, want 0", i, wr.cntBuf)
		}

		// Read some symbols.
		for i := range syms {
			sym, ok := rd.TryReadSymbol(&pd)
			if !ok {
				sym = rd.ReadSymbol(&pd)
			}
			if sym != syms[i] {
				t.Errorf("test %d, read back wrong symbol: got %d, want %d", i, sym, syms[i])
			}
			if rd.numBits >= 8 {
				t.Errorf("test %d, residual bits remaining: got %d, want < 8", i, rd.numBits)
			}
		}
		pads := rd.ReadPads()
		if _, err := rd.Flush(); err != nil {
			t.Errorf("test %d, unexpected Reader error: %v", i, err)
		}

		// Verify some Reader statistics.
		if pads != 0 {
			t.Errorf("test %d, unexpected padding bits: got %d, want 0", i, pads)
		}
		if rd.numBits != 0 {
			t.Errorf("test %d, residual bits remaining: got %d, want 0", i, rd.numBits)
		}
		if rd.Offset != wr.Offset {
			t.Errorf("test %d, offset mismatch: got %d, want %d", i, rd.Offset, wr.Offset)
		}
	}
}

func TestRange(t *testing.T) {
	vectors := []struct {
		input RangeCodes
		valid bool
	}{{
		input: RangeCodes{},
		valid: false,
	}, {
		input: RangeCodes{{5, 2}, {10, 5}}, // Gap in-between
		valid: false,
	}, {
		input: RangeCodes{{5, 20}, {7, 5}}, // All-encompassing overlap
		valid: false,
	}, {
		input: RangeCodes{{7, 5}, {5, 2}}, // Out-of-order
		valid: false,
	}, {
		input: RangeCodes{{5, 10}, {6, 11}}, // Forward-overlap is okay
		valid: true,
	}, {
		input: testRanges,
		valid: true,
	}, {
		input: MakeRangeCodes(0, []uint{
			0, 0, 0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 7, 8, 9, 10, 12, 14, 24,
		}),
		valid: true,
	}, {
		input: MakeRangeCodes(2, []uint{
			0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 7, 8, 9, 10, 24,
		}),
		valid: true,
	}, {
		input: MakeRangeCodes(1, []uint{
			2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 6, 6, 7, 8, 9, 10, 11, 12, 13, 24,
		}),
		valid: true,
	}, {
		input: MakeRangeCodes(2, []uint{
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		}),
		valid: true,
	}, {
		input: append(MakeRangeCodes(3, []uint{
			0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5,
		}), RangeCode{Base: 258, Len: 0}),
		valid: true,
	}, {
		input: MakeRangeCodes(1, []uint{
			0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12, 13, 13,
		}),
		valid: true,
	}}

	r := testutil.NewRand(0)
	for i, v := range vectors {
		if valid := v.input.checkValid(); valid != v.valid {
			t.Errorf("test %d, validity mismatch: got %v, want %v", i, valid, v.valid)
		}
		if !v.valid {
			continue // No point further testing invalid ranges
		}

		var re RangeEncoder
		re.Init(v.input)

		for _, rc := range v.input {
			offset := rc.Base + uint32(r.Intn(int(rc.End()-rc.Base)))
			sym := re.Encode(uint(offset))
			if int(sym) >= len(v.input) {
				t.Errorf("test %d, invalid symbol: re.Encode(%d) = %d", i, offset, sym)
			}
			rc := v.input[sym]
			if offset < rc.Base || offset >= rc.End() {
				t.Errorf("test %d, symbol not in range: %d not in %d..%d", i, offset, rc.Base, rc.End()-1)
			}
		}
	}
}

func BenchmarkBitReader(b *testing.B) {
	var br Reader
	nbs := []uint{1, 2, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 6, 6, 7, 7, 8, 9, 9, 13, 15}
	n := 16 * b.N
	bb := bytes.NewBuffer(make([]byte, n))
	br.Init(bb, false)

	b.SetBytes(16)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, nb := range nbs {
			_, ok := br.TryReadBits(nb)
			if !ok {
				_ = br.ReadBits(nb)
			}
		}
	}
}

func BenchmarkBitWriter(b *testing.B) {
	var bw Writer
	nbs := []uint{1, 2, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 6, 6, 7, 7, 8, 9, 9, 13, 15}
	n := 16 * b.N
	bb := bytes.NewBuffer(make([]byte, 0, n))
	bw.Init(bb, false)

	b.SetBytes(16)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, nb := range nbs {
			ok := bw.TryWriteBits(0, nb)
			if !ok {
				bw.WriteBits(0, nb)
			}
		}
	}
}
