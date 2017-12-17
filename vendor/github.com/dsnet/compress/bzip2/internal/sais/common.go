// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package sais implements a linear time suffix array algorithm.
package sais

//go:generate go run sais_gen.go byte sais_byte.go
//go:generate go run sais_gen.go int sais_int.go

// This package ports the C sais implementation by Yuta Mori. The ports are
// located in sais_byte.go and sais_int.go, which are identical to each other
// except for the types. Since Go does not support generics, we use generators to
// create the two files.
//
// References:
//	https://sites.google.com/site/yuta256/sais
//	https://ge-nong.googlecode.com/files/Linear%20Time%20Suffix%20Array%20Construction%20Using%20D-Critical%20Substrings.pdf
//	https://ge-nong.googlecode.com/files/Two%20Efficient%20Algorithms%20for%20Linear%20Time%20Suffix%20Array%20Construction.pdf

// ComputeSA computes the suffix array of t and places the result in sa.
// Both t and sa must be the same length.
func ComputeSA(t []byte, sa []int) {
	if len(sa) != len(t) {
		panic("mismatching sizes")
	}
	computeSA_byte(t, sa, 0, len(t), 256)
}
