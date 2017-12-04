// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"reflect"
	"testing"

	"github.com/dsnet/compress/internal/errors"
)

func TestMoveToFront(t *testing.T) {
	getDict := func(buf []byte) []uint8 {
		var dictMap [256]bool
		for _, b := range buf {
			dictMap[b] = true
		}
		var dictArr [256]uint8
		dict := dictArr[:0]
		for j, b := range dictMap {
			if b {
				dict = append(dict, uint8(j))
			}
		}
		return dict
	}

	vectors := []struct {
		size   int // If zero, default to 1MiB
		input  []byte
		output []uint16
		fail   bool
	}{{
		input:  []byte{},
		output: []uint16{},
	}, {
		input:  []byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
		output: []uint16{1, 1, 0},
	}, {
		input:  []byte{9, 8, 7, 6, 5, 4, 3, 2, 1},
		output: []uint16{9, 9, 9, 9, 9, 9, 9, 9, 9},
	}, {
		input:  []byte{42, 47, 42, 47, 42, 47, 42, 47, 42, 47, 42, 47},
		output: []uint16{0, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
	}, {
		input:  []byte{0, 5, 2, 3, 4, 4, 3, 1, 2, 3, 3, 3, 3, 3, 3, 4, 4, 4, 5, 2, 3, 3},
		output: []uint16{0, 6, 4, 5, 6, 0, 2, 6, 4, 3, 0, 1, 4, 1, 5, 4, 4, 0},
	}, {
		input:  []byte{100, 111, 108, 104, 10, 114, 101, 108, 108, 119, 111, 32},
		output: []uint16{3, 7, 7, 7, 5, 8, 8, 5, 0, 9, 7, 9},
	}, {
		input: []byte{
			103, 33, 107, 121, 110, 120, 101, 100, 101, 114, 44, 100, 111, 10, 32,
			108, 32, 105, 101, 108, 32, 104, 104, 112, 72, 118, 32, 111, 116, 84,
			117, 32, 99, 32, 114, 101, 108, 117, 119, 108, 100, 119, 32, 114, 102,
			109, 32, 101, 111, 98, 32, 113, 106, 111, 111, 32, 111, 122, 97,
		},
		output: []uint16{
			13, 4, 17, 30, 21, 30, 16, 16, 2, 26, 12, 4, 24, 12, 13, 23, 2, 22, 9,
			4, 4, 22, 0, 25, 18, 29, 5, 10, 28, 21, 29, 5, 25, 2, 17, 13, 13, 6, 30,
			3, 17, 3, 7, 7, 27, 29, 4, 9, 13, 28, 4, 30, 30, 5, 0, 4, 2, 31, 31,
		},
	}, {
		input: []byte{
			74, 69, 205, 44, 38, 175, 207, 101, 59, 108, 42, 155, 208, 50, 38, 115,
			190, 138, 163, 35, 13, 172, 160, 74, 68, 173, 99, 57, 213, 158, 248,
			209, 176, 52, 135, 21, 26, 248, 186, 186, 219, 113, 172, 163, 13, 22,
			100, 134, 4, 141, 53, 244, 99, 126, 214, 59, 53, 43, 146, 67, 131, 51,
			212, 146, 245,
		},
		output: []uint16{20, 20, 44, 13, 11, 41, 45, 26, 22, 27, 17, 37, 46, 21,
			10, 31, 46, 37, 42, 24, 21, 43, 43, 22, 33, 44, 35, 34, 49, 45, 54,
			49, 48, 38, 46, 35, 37, 7, 49, 0, 52, 45, 19, 22, 21, 40, 45, 48, 42,
			49, 46, 53, 24, 49, 53, 41, 6, 48, 52, 51, 52, 52, 53, 5, 54,
		},
	}, {
		input: []byte{
			153, 45, 45, 38, 135, 179, 26, 154, 165, 170, 170, 170, 170, 18, 109,
			240, 174, 150, 87, 164, 30, 30, 30, 30, 30, 30, 30, 148, 190, 10, 60,
			13, 13, 13, 13, 13, 6, 81, 200, 13, 225, 32, 17, 43, 22, 179, 13, 13,
			17, 236, 236, 236, 236, 236, 236, 236, 121, 211, 2, 211, 185, 54, 16,
			5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 50,
			5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 40,
		},
		output: []uint16{
			27, 17, 0, 15, 25, 33, 15, 29, 31, 32, 0, 0, 17, 28, 40, 34, 33, 31,
			34, 25, 1, 1, 34, 36, 23, 33, 25, 1, 0, 25, 34, 37, 4, 39, 32, 31, 34,
			33, 26, 7, 0, 5, 40, 1, 1, 38, 40, 34, 2, 40, 40, 38, 38, 0, 1, 1, 0,
			40, 2, 0, 1, 1, 0, 40,
		},
	}, {
		size:   10,
		input:  []byte{1, 1, 1, 1, 1, 2, 2, 2, 3, 3},
		output: []uint16{0, 1, 2, 1, 3, 0},
		fail:   false,
	}, {
		size:   10,
		input:  []byte{1, 1, 1, 1, 1, 2, 2, 2, 3, 3},
		output: []uint16{0, 1, 2, 1, 3, 1},
		fail:   true,
	}, {
		size:   10,
		input:  []byte{1, 1, 1, 1, 1, 2, 2, 2, 3, 3},
		output: []uint16{0, 1, 2, 1, 3, 2, 2},
		fail:   true,
	}, {
		size:   10,
		input:  []byte{1, 1, 1, 1, 1, 2, 2, 2, 3, 3},
		output: []uint16{1, 1, 2, 1, 3, 0},
		fail:   true,
	}, {
		size:  9,
		input: []byte{1, 1, 1, 1, 1, 2, 2, 2, 3, 3},
		fail:  true,
	}}

	mtf := new(moveToFront)
	for i, v := range vectors {
		var err error
		var input []byte
		var output []uint16
		func() {
			defer errors.Recover(&err)
			if v.size == 0 {
				v.size = 1 << 20
			}
			dict := getDict(v.input)
			mtf.Init(dict, v.size)
			output = mtf.Encode(v.input)
			mtf.Init(dict, v.size)
			input = mtf.Decode(v.output)
		}()

		fail := err != nil
		if fail && !v.fail {
			t.Errorf("test %d, unexpected error: %v", i, err)
		}
		if !fail && v.fail {
			t.Errorf("test %d, unexpected success", i)
		}
		if fail || v.fail {
			continue
		}
		if !reflect.DeepEqual(input, v.input) && !(len(input) == 0 && len(v.input) == 0) {
			t.Errorf("test %d, input mismatch:\ngot  %v\nwant %v", i, input, v.input)
		}
		if !reflect.DeepEqual(output, v.output) && !(len(output) == 0 && len(v.output) == 0) {
			t.Errorf("test %d, output mismatch:\ngot  %v\nwant %v", i, output, v.output)
		}
	}
}
