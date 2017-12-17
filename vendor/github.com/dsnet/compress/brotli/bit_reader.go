// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

import (
	"bufio"
	"io"

	"github.com/dsnet/compress/internal/errors"
)

// The bitReader preserves the property that it will never read more bytes than
// is necessary. However, this feature dramatically hurts performance because
// every byte needs to be obtained through a ReadByte method call.
// Furthermore, the decoding of variable length codes in ReadSymbol, often
// requires multiple passes before it knows the exact bit-length of the code.
//
// Thus, to improve performance, if the underlying byteReader is a bufio.Reader,
// then the bitReader will use the Peek and Discard methods to fill the internal
// bit buffer with as many bits as possible, allowing the TryReadBits and
// TryReadSymbol methods to often succeed on the first try.

type byteReader interface {
	io.Reader
	io.ByteReader
}

type bitReader struct {
	rd      byteReader
	bufBits uint64 // Buffer to hold some bits
	numBits uint   // Number of valid bits in bufBits
	offset  int64  // Number of bytes read from the underlying io.Reader

	// These fields are only used if rd is a bufio.Reader.
	bufRd       *bufio.Reader
	bufPeek     []byte // Buffer for the Peek data
	discardBits int    // Number of bits to discard from bufio.Reader
	fedBits     uint   // Number of bits fed in last call to FeedBits

	// Local copy of decoders to reduce memory allocations.
	prefix prefixDecoder
}

func (br *bitReader) Init(r io.Reader) {
	*br = bitReader{prefix: br.prefix}
	if rr, ok := r.(byteReader); ok {
		br.rd = rr
	} else {
		br.rd = bufio.NewReader(r)
	}
	if brd, ok := br.rd.(*bufio.Reader); ok {
		br.bufRd = brd
	}
}

// FlushOffset updates the read offset of the underlying byteReader.
// If the byteReader is a bufio.Reader, then this calls Discard to update the
// read offset.
func (br *bitReader) FlushOffset() int64 {
	if br.bufRd == nil {
		return br.offset
	}

	// Update the number of total bits to discard.
	br.discardBits += int(br.fedBits - br.numBits)
	br.fedBits = br.numBits

	// Discard some bytes to update read offset.
	nd := (br.discardBits + 7) / 8 // Round up to nearest byte
	nd, _ = br.bufRd.Discard(nd)
	br.discardBits -= nd * 8 // -7..0
	br.offset += int64(nd)

	// These are invalid after Discard.
	br.bufPeek = nil
	return br.offset
}

// FeedBits ensures that at least nb bits exist in the bit buffer.
// If the underlying byteReader is a bufio.Reader, then this will fill the
// bit buffer with as many bits as possible, relying on Peek and Discard to
// properly advance the read offset. Otherwise, it will use ReadByte to fill the
// buffer with just the right number of bits.
func (br *bitReader) FeedBits(nb uint) {
	if br.bufRd != nil {
		br.discardBits += int(br.fedBits - br.numBits)
		for {
			if len(br.bufPeek) == 0 {
				br.fedBits = br.numBits // Don't discard bits just added
				br.FlushOffset()

				var err error
				cntPeek := 8 // Minimum Peek amount to make progress
				if br.bufRd.Buffered() > cntPeek {
					cntPeek = br.bufRd.Buffered()
				}
				br.bufPeek, err = br.bufRd.Peek(cntPeek)
				br.bufPeek = br.bufPeek[int(br.numBits/8):] // Skip buffered bits
				if len(br.bufPeek) == 0 {
					if br.numBits >= nb {
						break
					}
					if err == io.EOF {
						err = io.ErrUnexpectedEOF
					}
					errors.Panic(err)
				}
			}
			cnt := int(64-br.numBits) / 8
			if cnt > len(br.bufPeek) {
				cnt = len(br.bufPeek)
			}
			for _, c := range br.bufPeek[:cnt] {
				br.bufBits |= uint64(c) << br.numBits
				br.numBits += 8
			}
			br.bufPeek = br.bufPeek[cnt:]
			if br.numBits > 56 {
				break
			}
		}
		br.fedBits = br.numBits
	} else {
		for br.numBits < nb {
			c, err := br.rd.ReadByte()
			if err != nil {
				if err == io.EOF {
					err = io.ErrUnexpectedEOF
				}
				errors.Panic(err)
			}
			br.bufBits |= uint64(c) << br.numBits
			br.numBits += 8
			br.offset++
		}
	}
}

// Read reads up to len(buf) bytes into buf.
func (br *bitReader) Read(buf []byte) (cnt int, err error) {
	if br.numBits%8 != 0 {
		return 0, errorf(errors.Invalid, "non-aligned bit buffer")
	}
	if br.numBits > 0 {
		for cnt = 0; len(buf) > cnt && br.numBits > 0; cnt++ {
			buf[cnt] = byte(br.bufBits)
			br.bufBits >>= 8
			br.numBits -= 8
		}
	} else {
		br.FlushOffset()
		cnt, err = br.rd.Read(buf)
		br.offset += int64(cnt)
	}
	return cnt, err
}

// TryReadBits attempts to read nb bits using the contents of the bit buffer
// alone. It returns the value and whether it succeeded.
//
// This method is designed to be inlined for performance reasons.
func (br *bitReader) TryReadBits(nb uint) (uint, bool) {
	if br.numBits < nb {
		return 0, false
	}
	val := uint(br.bufBits & uint64(1<<nb-1))
	br.bufBits >>= nb
	br.numBits -= nb
	return val, true
}

// ReadBits reads nb bits in LSB order from the underlying reader.
func (br *bitReader) ReadBits(nb uint) uint {
	br.FeedBits(nb)
	val := uint(br.bufBits & uint64(1<<nb-1))
	br.bufBits >>= nb
	br.numBits -= nb
	return val
}

// ReadPads reads 0-7 bits from the bit buffer to achieve byte-alignment.
func (br *bitReader) ReadPads() uint {
	nb := br.numBits % 8
	val := uint(br.bufBits & uint64(1<<nb-1))
	br.bufBits >>= nb
	br.numBits -= nb
	return val
}

// TryReadSymbol attempts to decode the next symbol using the contents of the
// bit buffer alone. It returns the decoded symbol and whether it succeeded.
//
// This method is designed to be inlined for performance reasons.
func (br *bitReader) TryReadSymbol(pd *prefixDecoder) (uint, bool) {
	if br.numBits < uint(pd.minBits) || len(pd.chunks) == 0 {
		return 0, false
	}
	chunk := pd.chunks[uint32(br.bufBits)&pd.chunkMask]
	nb := uint(chunk & prefixCountMask)
	if nb > br.numBits || nb > uint(pd.chunkBits) {
		return 0, false
	}
	br.bufBits >>= nb
	br.numBits -= nb
	return uint(chunk >> prefixCountBits), true
}

// ReadSymbol reads the next prefix symbol using the provided prefixDecoder.
func (br *bitReader) ReadSymbol(pd *prefixDecoder) uint {
	if len(pd.chunks) == 0 {
		errors.Panic(errInvalid) // Decode with empty tree
	}

	nb := uint(pd.minBits)
	for {
		br.FeedBits(nb)
		chunk := pd.chunks[uint32(br.bufBits)&pd.chunkMask]
		nb = uint(chunk & prefixCountMask)
		if nb > uint(pd.chunkBits) {
			linkIdx := chunk >> prefixCountBits
			chunk = pd.links[linkIdx][uint32(br.bufBits>>pd.chunkBits)&pd.linkMask]
			nb = uint(chunk & prefixCountMask)
		}
		if nb <= br.numBits {
			br.bufBits >>= nb
			br.numBits -= nb
			return uint(chunk >> prefixCountBits)
		}
	}
}

// ReadOffset reads an offset value using the provided rangesCodes indexed by
// the given symbol.
func (br *bitReader) ReadOffset(sym uint, rcs []rangeCode) uint {
	rc := rcs[sym]
	return uint(rc.base) + br.ReadBits(uint(rc.bits))
}

// ReadPrefixCode reads the prefix definition from the stream and initializes
// the provided prefixDecoder. The value maxSyms is the alphabet size of the
// prefix code being generated. The actual number of representable symbols
// will be between 1 and maxSyms, inclusively.
func (br *bitReader) ReadPrefixCode(pd *prefixDecoder, maxSyms uint) {
	hskip := br.ReadBits(2)
	if hskip == 1 {
		br.readSimplePrefixCode(pd, maxSyms)
	} else {
		br.readComplexPrefixCode(pd, maxSyms, hskip)
	}
}

// readSimplePrefixCode reads the prefix code according to RFC section 3.4.
func (br *bitReader) readSimplePrefixCode(pd *prefixDecoder, maxSyms uint) {
	var codes [4]prefixCode
	nsym := int(br.ReadBits(2)) + 1
	clen := neededBits(uint32(maxSyms))
	for i := 0; i < nsym; i++ {
		codes[i].sym = uint32(br.ReadBits(clen))
	}

	copyLens := func(lens []uint) {
		for i := 0; i < nsym; i++ {
			codes[i].len = uint32(lens[i])
		}
	}
	compareSwap := func(i, j int) {
		if codes[i].sym > codes[j].sym {
			codes[i], codes[j] = codes[j], codes[i]
		}
	}

	switch nsym {
	case 1:
		copyLens(simpleLens1[:])
	case 2:
		copyLens(simpleLens2[:])
		compareSwap(0, 1)
	case 3:
		copyLens(simpleLens3[:])
		compareSwap(0, 1)
		compareSwap(0, 2)
		compareSwap(1, 2)
	case 4:
		if tsel := br.ReadBits(1) == 1; !tsel {
			copyLens(simpleLens4a[:])
		} else {
			copyLens(simpleLens4b[:])
		}
		compareSwap(0, 1)
		compareSwap(2, 3)
		compareSwap(0, 2)
		compareSwap(1, 3)
		compareSwap(1, 2)
	}
	if uint(codes[nsym-1].sym) >= maxSyms {
		errors.Panic(errCorrupted) // Symbol goes beyond range of alphabet
	}
	pd.Init(codes[:nsym], true) // Must have 1..4 symbols
}

// readComplexPrefixCode reads the prefix code according to RFC section 3.5.
func (br *bitReader) readComplexPrefixCode(pd *prefixDecoder, maxSyms, hskip uint) {
	// Read the code-lengths prefix table.
	var codeCLensArr [len(complexLens)]prefixCode // Sorted, but may have holes
	sum := 32
	for _, sym := range complexLens[hskip:] {
		clen := br.ReadSymbol(&decCLens)
		if clen > 0 {
			codeCLensArr[sym] = prefixCode{sym: uint32(sym), len: uint32(clen)}
			if sum -= 32 >> clen; sum <= 0 {
				break
			}
		}
	}
	codeCLens := codeCLensArr[:0] // Compact the array to have no holes
	for _, c := range codeCLensArr {
		if c.len > 0 {
			codeCLens = append(codeCLens, c)
		}
	}
	if len(codeCLens) < 1 {
		errors.Panic(errCorrupted)
	}
	br.prefix.Init(codeCLens, true) // Must have 1..len(complexLens) symbols

	// Use code-lengths table to decode rest of prefix table.
	var codesArr [maxNumAlphabetSyms]prefixCode
	var sym, repSymLast, repCntLast, clenLast uint = 0, 0, 0, 8
	codes := codesArr[:0]
	for sym, sum = 0, 32768; sym < maxSyms && sum > 0; {
		clen := br.ReadSymbol(&br.prefix)
		if clen < 16 {
			// Literal bit-length symbol used.
			if clen > 0 {
				codes = append(codes, prefixCode{sym: uint32(sym), len: uint32(clen)})
				clenLast = clen
				sum -= 32768 >> clen
			}
			repSymLast = 0 // Reset last repeater symbol
			sym++
		} else {
			// Repeater symbol used.
			//	16: Repeat previous non-zero code-length
			//	17: Repeat code length of zero

			repSym := clen // Rename clen for better clarity
			if repSym != repSymLast {
				repCntLast = 0
				repSymLast = repSym
			}

			nb := repSym - 14          // 2..3 bits
			rep := br.ReadBits(nb) + 3 // 3..6 or 3..10
			if repCntLast > 0 {
				rep += (repCntLast - 2) << nb // Modify previous repeat count
			}
			repDiff := rep - repCntLast // Always positive
			repCntLast = rep

			if repSym == 16 {
				clen := clenLast
				for symEnd := sym + repDiff; sym < symEnd; sym++ {
					codes = append(codes, prefixCode{sym: uint32(sym), len: uint32(clen)})
				}
				sum -= int(repDiff) * (32768 >> clen)
			} else {
				sym += repDiff
			}
		}
	}
	if len(codes) < 2 || sym > maxSyms {
		errors.Panic(errCorrupted)
	}
	pd.Init(codes, true) // Must have 2..maxSyms symbols
}
