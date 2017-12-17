// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build !no_lib_uk

package main

import (
	"io"
	"io/ioutil"

	"github.com/ulikunitz/xz/lzma"
)

func init() {
	RegisterEncoder(FormatLZMA2, "uk",
		func(w io.Writer, lvl int) io.WriteCloser {
			// This level conversion logic emulates the conversion found in
			// LZMA2Options.java from http://git.tukaani.org/xz-java.git.
			if lvl < 0 || lvl > 9 {
				panic("invalid level")
			}
			dict := [...]int{
				1 << 18, 1 << 20, 1 << 21, 1 << 22, 1 << 22,
				1 << 23, 1 << 23, 1 << 24, 1 << 25, 1 << 26,
			}[lvl]
			match := lzma.HashTable4
			// TODO(dsnet): This currently crashes on zero.bin when using
			// BinaryTree on revision 76f94b7c69c6f84be96bcfc2443042b198689565.
			/*
				if lvl > 4 {
					match = lzma.BinaryTree
				}
			*/

			zw, err := lzma.Writer2Config{DictCap: dict, Matcher: match}.NewWriter2(w)
			if err != nil {
				panic(err)
			}
			return zw
		})
	RegisterDecoder(FormatLZMA2, "uk",
		func(r io.Reader) io.ReadCloser {
			zr, err := lzma.NewReader2(r)
			if err != nil {
				panic(err)
			}
			return ioutil.NopCloser(zr)
		})
}
