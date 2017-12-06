// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package brotli implements the Brotli compressed data format,
// described in RFC 7932.
package brotli

import (
	"fmt"

	"github.com/dsnet/compress/internal/errors"
)

func errorf(c int, f string, a ...interface{}) error {
	return errors.Error{Code: c, Pkg: "brotli", Msg: fmt.Sprintf(f, a...)}
}

// errWrap converts a lower-level errors.Error to be one from this package.
// The replaceCode passed in will be used to replace the code for any errors
// with the errors.Invalid code.
//
// For the Reader, set this to errors.Corrupted.
// For the Writer, set this to errors.Internal.
func errWrap(err error, replaceCode int) error {
	if cerr, ok := err.(errors.Error); ok {
		if errors.IsInvalid(cerr) {
			cerr.Code = replaceCode
		}
		err = errorf(cerr.Code, "%s", cerr.Msg)
	}
	return err
}

var (
	errClosed    = errorf(errors.Closed, "")
	errCorrupted = errorf(errors.Corrupted, "")
	errInvalid   = errorf(errors.Invalid, "")
	errUnaligned = errorf(errors.Invalid, "non-aligned bit buffer")
)

var (
	reverseLUT [256]uint8
)

func init() {
	initLUTs()
}

func initLUTs() {
	initCommonLUTs()
	initPrefixLUTs()
	initContextLUTs()
	initDictLUTs()
}

func initCommonLUTs() {
	for i := range reverseLUT {
		b := uint8(i)
		b = (b&0xaa)>>1 | (b&0x55)<<1
		b = (b&0xcc)>>2 | (b&0x33)<<2
		b = (b&0xf0)>>4 | (b&0x0f)<<4
		reverseLUT[i] = b
	}
}

// neededBits computes the minimum number of bits needed to encode n elements.
func neededBits(n uint32) (nb uint) {
	for n--; n > 0; n >>= 1 {
		nb++
	}
	return
}

// reverseUint32 reverses all bits of v.
func reverseUint32(v uint32) (x uint32) {
	x |= uint32(reverseLUT[byte(v>>0)]) << 24
	x |= uint32(reverseLUT[byte(v>>8)]) << 16
	x |= uint32(reverseLUT[byte(v>>16)]) << 8
	x |= uint32(reverseLUT[byte(v>>24)]) << 0
	return x
}

// reverseBits reverses the lower n bits of v.
func reverseBits(v uint32, n uint) uint32 {
	return reverseUint32(v << (32 - n))
}

func allocUint8s(s []uint8, n int) []uint8 {
	if cap(s) >= n {
		return s[:n]
	}
	return make([]uint8, n, n*3/2)
}

func allocUint32s(s []uint32, n int) []uint32 {
	if cap(s) >= n {
		return s[:n]
	}
	return make([]uint32, n, n*3/2)
}

func extendSliceUints32s(s [][]uint32, n int) [][]uint32 {
	if cap(s) >= n {
		return s[:n]
	}
	ss := make([][]uint32, n, n*3/2)
	copy(ss, s[:cap(s)])
	return ss
}

func extendDecoders(s []prefixDecoder, n int) []prefixDecoder {
	if cap(s) >= n {
		return s[:n]
	}
	ss := make([]prefixDecoder, n, n*3/2)
	copy(ss, s[:cap(s)])
	return ss
}
