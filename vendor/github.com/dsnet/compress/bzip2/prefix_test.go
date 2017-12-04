// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"reflect"
	"testing"

	"github.com/dsnet/compress/internal/prefix"
)

func TestDegenerateCodes(t *testing.T) {
	vectors := []struct {
		input  prefix.PrefixCodes
		output prefix.PrefixCodes
	}{{
		input: []prefix.PrefixCode{
			{Sym: 0, Len: 1},
		},
		output: []prefix.PrefixCode{
			{Sym: 0, Len: 1, Val: 0},   // 0
			{Sym: 258, Len: 1, Val: 1}, // 1
		},
	}, {
		input: []prefix.PrefixCode{
			{Sym: 0, Len: 1},
			{Sym: 1, Len: 1},
			{Sym: 2, Len: 1},
		},
		output: []prefix.PrefixCode{
			{Sym: 0, Len: 1, Val: 0}, // 0
			{Sym: 1, Len: 1, Val: 1}, // 1
		},
	}, {
		input: []prefix.PrefixCode{
			{Sym: 0, Len: 3},
			{Sym: 1, Len: 4},
			{Sym: 2, Len: 3},
		},
		output: []prefix.PrefixCode{
			{Sym: 0, Len: 3, Val: 0},    //  000
			{Sym: 1, Len: 4, Val: 2},    // 0010
			{Sym: 2, Len: 3, Val: 4},    //  100
			{Sym: 258, Len: 4, Val: 10}, // 1010
			{Sym: 259, Len: 3, Val: 6},  //  110
			{Sym: 260, Len: 1, Val: 1},  //    1
		},
	}, {
		input: []prefix.PrefixCode{
			{Sym: 0, Len: 1},
			{Sym: 1, Len: 3},
			{Sym: 2, Len: 4},
			{Sym: 3, Len: 3},
			{Sym: 4, Len: 2},
		},
		output: []prefix.PrefixCode{
			{Sym: 0, Len: 1, Val: 0}, //   0
			{Sym: 1, Len: 3, Val: 3}, // 011
			{Sym: 3, Len: 3, Val: 7}, // 111
			{Sym: 4, Len: 2, Val: 1}, //  01
		},
	}}

	for i, v := range vectors {
		input := append(prefix.PrefixCodes(nil), v.input...)
		output := handleDegenerateCodes(input)

		if !reflect.DeepEqual(output, v.output) {
			t.Errorf("test %d, output mismatch:\ngot  %v\nwant %v", i, output, v.output)
		}
	}
}
