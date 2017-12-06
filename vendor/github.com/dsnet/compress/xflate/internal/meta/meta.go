// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package meta implements the XFLATE meta encoding scheme.
//
// The XFLATE meta encoding is a method of encoding arbitrary data into one
// or more RFC 1951 compliant DEFLATE blocks. This encoding has the special
// property that when the blocks are decoded by a RFC 1951 compliant
// decompressor, they produce absolutely no output. However, when decoded with
// the XFLATE meta decoder, it losslessly produces the original input.
//
// The meta encoding works by encoding arbitrary data into the Huffman tree
// definition of dynamic DEFLATE blocks. These blocks have an empty data section
// and produce no output. Due to the Huffman definition overhead, the encoded
// output is usually larger than the input. However, for most input datasets,
// this encoding scheme is able to achieve an efficiency of at least 50%.
package meta

import (
	"fmt"

	"github.com/dsnet/compress/internal/errors"
	"github.com/dsnet/compress/internal/prefix"
)

// These are the magic values that begin every single meta block.
const (
	magicVals uint32 = 0x05860004
	magicMask uint32 = 0xfffe3fc6
)

// ReverseSearch searches for a meta header in reverse. This returns the last
// index where the header was found. If not found, it returns -1.
func ReverseSearch(data []byte) int {
	var magic uint32
	for i := len(data) - 1; i >= 0; i-- {
		magic = (magic << 8) | uint32(data[i])
		if magic&magicMask == magicVals {
			return i
		}
	}
	return -1
}

const (
	maxSyms    = 257 // Maximum number of literal codes (with EOB marker)
	minHuffLen = 1   // Minimum number of bits for each Huffman code
	maxHuffLen = 7   // Maximum number of bits for each Huffman code
	minRepLast = 3   // Minimum number of repeated codes (clen: 16)
	maxRepLast = 6   // Maximum number of repeated codes (clen: 16)
	minRepZero = 11  // Minimum number of repeated zeros (clen: 18)
	maxRepZero = 138 // Maximum number of repeated zeros (clen: 18)
)

// These are some constants regarding the theoretical and practical limits for
// the meta encoding of a single block.
const (
	// MinRawBytes and MaxRawBytes are the theoretical minimum and maximum
	// number of bytes a block can encode.
	MinRawBytes = 0
	MaxRawBytes = 31

	// MinEncBytes and MaxEncBytes are the theoretical minimum and maximum
	// number of bytes an encoded block will occupy.
	MinEncBytes = 12
	MaxEncBytes = 64

	// EnsureRawBytes is the maximum number of bytes that a single block is
	// ensured to encode (computed using brute force).
	EnsureRawBytes = 22
)

// FinalMode controls or indicates which final bits are set in the last block
// in the meta stream. In the meta encoding, there are 2 final bits:
//
//	Stream: This is the final bit from DEFLATE (as defined in RFC 1951).
//	and indicates that the entire compression stream has come to an end.
//	This bit indicate absolute termination of the stream.
//
//	Meta: This final bit indicates that the current sequence of meta blocks has
//	terminated. The decoded data from those blocks form a meta substream.
//	This bit is used as a form of message framing for the meta encoding format.
//
// It invalid for the stream final bit to be set, while the meta final bit is
// not set. All other combinations are legal.
type FinalMode int

const (
	// FinalNil has neither the stream nor meta final bits set.
	FinalNil FinalMode = iota

	// FinalMeta has the final bit set, but not stream final bit.
	FinalMeta

	// FinalStream has both the meta and stream final bits set.
	FinalStream
)

// The Huffman encoding used by the XFLATE meta encoding uses a partially
// pre-determined HCLEN table. The symbols are 0, 16, 18, and another symbol
// between minHuffLen and maxHuffLen, inclusively. The 0 symbol represents a
// logical zero in the meta encoding, and the symbol between minHuffLen and
// maxHuffLen represents a logical one. Symbols 16 and 18 are used to provide a
// primitive form of run-length encoding. The codes that these symbols map to
// are fixed and are shown below.
//
//	Code      Symbol
//	0    <=>  0                      (symZero)
//	10   <=>  minHuffLen..maxHuffLen (symOne)
//	110  <=>  16                     (symRepLast)
//	111  <=>  18                     (symRepZero)
//
// The symZero symbol occupies 1 bit since it is the most commonly needed bit,
// while symOne occupies 2 bits. Thus, it is cheaper to encode logical zeros
// than it is to encode logical ones. The invert bit in the meta encoding takes
// advantage of this fact and allows all data bits to be inverted so that the
// number zero bits is greater than the number of one bits.
const (
	symZero    = iota // Symbol to encode a input zero (clen: 0)
	symOne            // Symbol to encode a input one  (clen: minHuffLen..maxHuffLen)
	symRepLast        // Symbol to repeat last symbol  (clen: 16)
	symRepZero        // Symbol to repeat zero symbol  (clen: 18)
)

var encHuff, decHuff = func() (enc prefix.Encoder, dec prefix.Decoder) {
	codes := [4]prefix.PrefixCode{
		{Sym: symZero, Len: 1},
		{Sym: symOne, Len: 2},
		{Sym: symRepLast, Len: 3},
		{Sym: symRepZero, Len: 3},
	}
	prefix.GeneratePrefixes(codes[:])
	enc.Init(codes[:])
	dec.Init(codes[:])
	return
}()

func errorf(c int, f string, a ...interface{}) error {
	return errors.Error{Code: c, Pkg: "meta", Msg: fmt.Sprintf(f, a...)}
}

var errClosed = errorf(errors.Closed, "")

// oneBitsLUT reports the number of set bits in the input byte.
var oneBitsLUT = func() (lut [256]byte) {
	for i := range lut[:] {
		for b := byte(i); b > 0; b >>= 1 {
			lut[i] += b & 1
		}
	}
	return
}()

// numBits counts the number of zero and one bits in the byte.
func numBits(b byte) (zeros, ones int) {
	ones = int(oneBitsLUT[b])
	zeros = 8 - ones
	return
}

// numPads computes number of bits needed to pad n-bits to a byte alignment.
func numPads(n uint) uint {
	return -n & 7
}

// btoi converts a bool to a integer 0 or 1.
func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
