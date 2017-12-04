// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build ignore

// Generates huffman.txt. This test file heavily favors prefix based encodings
// since some symbols are heavily favored over others. This leads to compression
// savings that can be gained by assigning shorter prefix codes to those more
// frequent symbols. The number of symbols used is large enough such that it
// avoids LZ77 dictionary matches.
package main

import (
	"io/ioutil"
	"math/rand"
	"unicode/utf8"
)

const (
	name = "huffman.txt"
	size = 1 << 18
)

const (
	alpha1 = "abcdefghijklmnopqrstuvwxyz"
	alpha2 = alpha1 + "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	alpha3 = alpha2 + "0123456789" + "+/"
)

func main() {
	var b []byte
	r := rand.New(rand.NewSource(0))

	for len(b) < size {
		n := 16 + r.Intn(64) // Length of substring
		p := r.Float32()
		switch {
		case p <= 0.75:
			// Write strings of base64 encoded values.
			for i := 0; i < n; i++ {
				p := r.Float32()
				switch {
				case p <= 0.1:
					// Write any lowercase letter.
					b = append(b, alpha1[r.Intn(len(alpha1))])
				case p <= 0.7:
					// Write any lowercase or uppercase letter.
					b = append(b, alpha2[r.Intn(len(alpha2))])
				case p <= 1.0:
					// Write any character from the base64 alphabet.
					b = append(b, alpha3[r.Intn(len(alpha3))])
				}
			}
		case p <= 1.00:
			// Write strings of utf8 encoded values.
			for i := 0; i < n; i++ {
				p := r.Float32()
				switch {
				case p <= 0.65:
					// Write a 2-byte long utf8 code point.
					var buf [4]byte
					cnt := utf8.EncodeRune(buf[:], rune(0x80+r.Intn(0x780)))
					b = append(b, buf[:cnt]...)
				case p <= 1.00:
					// Write a 3-byte long utf8 code point.
					var buf [4]byte
					cnt := utf8.EncodeRune(buf[:], rune(0x800+r.Intn(0xF800)))
					b = append(b, buf[:cnt]...)
				}
			}
		}
	}

	if err := ioutil.WriteFile(name, b[:size], 0664); err != nil {
		panic(err)
	}
}
