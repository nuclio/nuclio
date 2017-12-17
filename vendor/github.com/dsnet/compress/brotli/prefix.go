// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

const (
	// RFC section 3.5.
	// This is the maximum bit-width of a prefix code.
	// Thus, it is okay to use uint32 to store codes.
	maxPrefixBits = 15

	// RFC section 3.3.
	// The size of the alphabet for various prefix codes.
	numLitSyms        = 256                  // Literal symbols
	maxNumDistSyms    = 16 + 120 + (48 << 3) // Distance symbols
	numIaCSyms        = 704                  // Insert-and-copy length symbols
	numBlkCntSyms     = 26                   // Block count symbols
	maxNumBlkTypeSyms = 256 + 2              // Block type symbols
	maxNumCtxMapSyms  = 256 + 16             // Context map symbols

	// This should be the max of each of the constants above.
	maxNumAlphabetSyms = numIaCSyms
)

var (
	// RFC section 3.4.
	// Prefix code lengths for simple codes.
	simpleLens1  = [1]uint{0}
	simpleLens2  = [2]uint{1, 1}
	simpleLens3  = [3]uint{1, 2, 2}
	simpleLens4a = [4]uint{2, 2, 2, 2}
	simpleLens4b = [4]uint{1, 2, 3, 3}

	// RFC section 3.5.
	// Prefix code lengths for complex codes as they appear in the stream.
	complexLens = [18]uint{
		1, 2, 3, 4, 0, 5, 17, 6, 16, 7, 8, 9, 10, 11, 12, 13, 14, 15,
	}
)

type rangeCode struct {
	base uint32 // Starting base offset of the range
	bits uint32 // Bit-width of a subsequent integer to add to base offset
}

var (
	// RFC section 5.
	// LUT to convert an insert symbol to an actual insert length.
	insLenRanges []rangeCode

	// RFC section 5.
	// LUT to convert an copy symbol to an actual copy length.
	cpyLenRanges []rangeCode

	// RFC section 6.
	// LUT to convert an block-type length symbol to an actual length.
	blkLenRanges []rangeCode

	// RFC section 7.3.
	// LUT to convert RLE symbol to an actual repeat length.
	maxRLERanges []rangeCode
)

type prefixCode struct {
	sym uint32 // The symbol being mapped
	val uint32 // Value of the prefix code (must be in [0..1<<len])
	len uint32 // Bit length of the prefix code
}

var (
	// RFC section 3.5.
	// Prefix codecs for code lengths in complex prefix definition.
	codeCLens []prefixCode
	decCLens  prefixDecoder
	encCLens  prefixEncoder

	// RFC section 7.3.
	// Prefix codecs for RLEMAX in context map definition.
	codeMaxRLE []prefixCode
	decMaxRLE  prefixDecoder
	encMaxRLE  prefixEncoder

	// RFC section 9.1.
	// Prefix codecs for WBITS in stream header definition.
	codeWinBits []prefixCode
	decWinBits  prefixDecoder
	encWinBits  prefixEncoder

	// RFC section 9.2.
	// Prefix codecs used for size fields in meta-block header definition.
	codeCounts []prefixCode
	decCounts  prefixDecoder
	encCounts  prefixEncoder
)

var (
	// RFC section 5.
	// Table to convert insert-and-copy symbols to insert and copy lengths.
	iacLUT [numIaCSyms]struct{ ins, cpy rangeCode }

	// RFC section 4.
	// Table to help convert short-codes (first 16 symbols) to distances using
	// the ring buffer of past distances.
	distShortLUT [16]struct{ index, delta int }

	// RFC section 4.
	// Table to help convert long-codes to distances. This is two dimensional
	// slice keyed by the NPOSTFIX and the normalized distance symbol.
	distLongLUT [4][]rangeCode
)

func initPrefixLUTs() {
	// Sanity check some constants.
	for _, numMax := range []uint{
		numLitSyms, maxNumDistSyms, numIaCSyms, numBlkCntSyms, maxNumBlkTypeSyms, maxNumCtxMapSyms,
	} {
		if numMax > maxNumAlphabetSyms {
			panic("maximum alphabet size is not updated")
		}
	}
	if maxNumAlphabetSyms >= 1<<prefixSymbolBits {
		panic("maximum alphabet size is too large to represent")
	}
	if maxPrefixBits >= 1<<prefixCountBits {
		panic("maximum prefix bit-length is too large to represent")
	}

	initPrefixRangeLUTs()
	initPrefixCodeLUTs()
	initLengthLUTs()
}

func initPrefixRangeLUTs() {
	makeRanges := func(base uint, bits []uint) (rc []rangeCode) {
		for _, nb := range bits {
			rc = append(rc, rangeCode{base: uint32(base), bits: uint32(nb)})
			base += 1 << nb
		}
		return rc
	}

	insLenRanges = makeRanges(0, []uint{
		0, 0, 0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 7, 8, 9, 10, 12, 14, 24,
	}) // RFC section 5
	cpyLenRanges = makeRanges(2, []uint{
		0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 7, 8, 9, 10, 24,
	}) // RFC section 5
	blkLenRanges = makeRanges(1, []uint{
		2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 6, 6, 7, 8, 9, 10, 11, 12, 13, 24,
	}) // RFC section 6
	maxRLERanges = makeRanges(2, []uint{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
	}) // RFC section 7.3
}

func initPrefixCodeLUTs() {
	// Prefix code for reading code lengths in RFC section 3.5.
	codeCLens = nil
	for sym, clen := range []uint{2, 4, 3, 2, 2, 4} {
		code := prefixCode{sym: uint32(sym), len: uint32(clen)}
		codeCLens = append(codeCLens, code)
	}
	decCLens.Init(codeCLens, true)
	encCLens.Init(codeCLens)

	// Prefix code for reading RLEMAX in RFC section 7.3.
	codeMaxRLE = []prefixCode{{sym: 0, val: 0, len: 1}}
	for i := uint32(0); i < 16; i++ {
		code := prefixCode{sym: i + 1, val: i<<1 | 1, len: 5}
		codeMaxRLE = append(codeMaxRLE, code)
	}
	decMaxRLE.Init(codeMaxRLE, false)
	encMaxRLE.Init(codeMaxRLE)

	// Prefix code for reading WBITS in RFC section 9.1.
	codeWinBits = nil
	for i := uint32(9); i <= 24; i++ {
		var code prefixCode
		switch {
		case i == 16:
			code = prefixCode{sym: i, val: (i-16)<<0 | 0, len: 1} // Symbols: 16
		case i > 17:
			code = prefixCode{sym: i, val: (i-17)<<1 | 1, len: 4} // Symbols: 18..24
		case i < 17:
			code = prefixCode{sym: i, val: (i-8)<<4 | 1, len: 7} // Symbols: 9..15
		default:
			code = prefixCode{sym: i, val: (i-17)<<4 | 1, len: 7} // Symbols: 17
		}
		codeWinBits = append(codeWinBits, code)
	}
	codeWinBits[0].sym = 0 // Invalid code "1000100" to use symbol zero
	decWinBits.Init(codeWinBits, false)
	encWinBits.Init(codeWinBits)

	// Prefix code for reading counts in RFC section 9.2.
	// This is used for: NBLTYPESL, NBLTYPESI, NBLTYPESD, NTREESL, and NTREESD.
	codeCounts = []prefixCode{{sym: 1, val: 0, len: 1}}
	code := codeCounts[len(codeCounts)-1]
	for i := uint32(0); i < 8; i++ {
		for j := uint32(0); j < 1<<i; j++ {
			code.sym = code.sym + 1
			code.val = j<<4 | i<<1 | 1
			code.len = i + 4
			codeCounts = append(codeCounts, code)
		}
	}
	decCounts.Init(codeCounts, false)
	encCounts.Init(codeCounts)
}

func initLengthLUTs() {
	// RFC section 5.
	// The insert-and-copy length symbol is converted into an insert length
	// and a copy length. Thus, create a table to precompute the result for
	// all input symbols.
	for iacSym := range iacLUT {
		var insSym, cpySym int
		switch iacSym / 64 {
		case 0, 2: // 0..63 and 128..191
			insSym, cpySym = 0, 0
		case 1, 3: // 64..127 and 192..255
			insSym, cpySym = 0, 8
		case 4: // 256..319
			insSym, cpySym = 8, 0
		case 5: // 320..383
			insSym, cpySym = 8, 8
		case 6: // 384..447
			insSym, cpySym = 0, 16
		case 7: // 448..511
			insSym, cpySym = 16, 0
		case 8: // 512..575
			insSym, cpySym = 8, 16
		case 9: // 576..639
			insSym, cpySym = 16, 8
		case 10: // 640..703
			insSym, cpySym = 16, 16
		}

		r64 := iacSym % 64
		insSym += r64 >> 3   // Lower 3 bits
		cpySym += r64 & 0x07 // Upper 3 bits

		iacLUT[iacSym].ins = insLenRanges[insSym]
		iacLUT[iacSym].cpy = cpyLenRanges[cpySym]
	}

	// RFC section 4.
	// The first 16 symbols modify a previously seen symbol. Thus, we can create
	// a table to determine which distance to use and how much to modify it by.
	for distSym := range distShortLUT {
		var index, delta int
		switch {
		case distSym < 4:
			index, delta = distSym, 0
		case distSym < 10:
			index, delta = 0, distSym/2-1
		case distSym < 16:
			index, delta = 1, distSym/2-4
		}
		if distSym%2 == 0 {
			delta *= -1
		}
		distShortLUT[distSym].index = index
		distShortLUT[distSym].delta = delta
	}

	// RFC section 4.
	// Longer distances are computed according the equation in the RFC.
	// To reduce computation during runtime, we precompute as much of the output
	// as possible. Thus, we compute the final distance using the following:
	//	rec := distLongLUT[NPOSTFIX][distSym - (16+NDIRECT)]
	//	distance := NDIRECT + rec.base + ReadBits(rec.bits)<<NPOSTFIX
	for npostfix := range distLongLUT {
		numDistSyms := 48 << uint(npostfix)
		distLongLUT[npostfix] = make([]rangeCode, numDistSyms)
		for distSym := range distLongLUT[npostfix] {
			postfixMask := 1<<uint(npostfix) - 1
			hcode := distSym >> uint(npostfix)
			lcode := distSym & postfixMask
			nbits := 1 + distSym>>uint(npostfix+1)
			offset := ((2 + (hcode & 1)) << uint(nbits)) - 4
			distLongLUT[npostfix][distSym] = rangeCode{
				base: uint32(offset<<uint(npostfix) + lcode + 1),
				bits: uint32(nbits),
			}
		}
	}
}
