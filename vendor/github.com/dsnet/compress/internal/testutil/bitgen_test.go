// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package testutil

import "testing"

func TestDecodeBitGen(t *testing.T) {
	vectors := []struct {
		input  string
		output []byte
		valid  bool
	}{{
		input: ``,
		valid: false,
	}, {
		input: `#<<<`,
		valid: false,
	}, {
		input: `<<<`,
		valid: true,
	}, {
		input: `<<< <`,
		valid: true,
	}, {
		input: `<<< <*5`,
		valid: false,
	}, {
		input:  `<<< < 1011001110001`,
		output: []byte{0x71, 0x16}, // 0b01110001 0b00010110
		valid:  true,
	}, {
		input:  `<<< > 1011001110001`,
		output: []byte{0xcd, 0x11}, // 0b11001101 0b00010001
		valid:  true,
	}, {
		input:  `>>> < 1011001110001`,
		output: []byte{0x8e, 0x68}, // 0b10001110 0b01101000
		valid:  true,
	}, {
		input:  `>>> > 1011001110001`,
		output: []byte{0xb3, 0x88}, // 0b10110011 0b10001000
		valid:  true,
	}, {
		input: `>>> > 1011001110001 <<<`,
		valid: false,
	}, {
		input: `<<< < 0111 X:`,
		valid: false,
	}, {
		input: `<<< < 0111 X:33`,
		valid: false,
	}, {
		input: `<<< < X:3`,
		valid: false,
	}, {
		input: `<<< meow`,
		valid: false,
	}, {
		input:  `<<< < X:0f1b`,
		output: []byte{0x0f, 0x1b},
		valid:  true,
	}, {
		input:  `<<< < H16:0f1b`,
		output: []byte{0x1b, 0x0f},
		valid:  true,
	}, {
		input:  `<<< < H12:0f1b`,
		output: []byte{0x1b, 0x0f},
		valid:  true,
	}, {
		input:  `<<< < H12:0f1b H12:0f1b`,
		output: []byte{0x1b, 0xbf, 0xf1},
		valid:  true,
	}, {
		input: `<<< < H12:0f1b H11:0f1b`,
		valid: false,
	}, {
		input: `>>>
			< 110 D64:18364758544493064720 # Comment
			> 110 D64:18364758544493064720 # Comment
		`,
		output: []byte{
			0x61, 0x09, 0x85, 0x4d, // 0b01100001 0b00001001 0b10000101 0b01001101
			0xc3, 0x2b, 0xa7, 0x6f, // 0b11000011 0b00101011 0b10100111 0b01101111
			0xfb, 0xfb, 0x72, 0xea, // 0b11111011 0b11111011 0b01110010 0b11101010
			0x61, 0xd9, 0x50, 0xc8, // 0b01100001 0b11011001 0b01010000 0b11001000
			0x40, // 0b01000000
		},
		valid: true,
	}, {
		input: `<<< < 1111111011011100101110101001100001110110010101000011001000010000`,
		output: []byte{
			0x10, 0x32, 0x54, 0x76, // 0b00010000 0b00110010 0b01010100 0b01110110
			0x98, 0xba, 0xdc, 0xfe, // 0b10011000 0b10111010 0b11011100 0b11111110
		},
		valid: true,
	}, {
		input: `<<< > 1111111011011100101110101001100001110110010101000011001000010000`,
		output: []byte{
			0x7f, 0x3b, 0x5d, 0x19, // 0b01111111 0b00111011 0b01011101 0b00011001
			0x6e, 0x2a, 0x4c, 0x08, // 0b01101110 0b00101010 0b01001100 0b00001000
		},
		valid: true,
	}, {
		input: `>>> < 1111111011011100101110101001100001110110010101000011001000010000`,
		output: []byte{
			0x08, 0x4c, 0x2a, 0x6e, // 0b00001000 0b01001100 0b00101010 0b01101110
			0x19, 0x5d, 0x3b, 0x7f, // 0b00011001 0b01011101 0b00111011 0b01111111
		},
		valid: true,
	}, {
		input: `>>> > 1111111011011100101110101001100001110110010101000011001000010000`,
		output: []byte{
			0xfe, 0xdc, 0xba, 0x98, // 0b11111110 0b11011100 0b10111010 0b10011000
			0x76, 0x54, 0x32, 0x10, // 0b01110110 0b01010100 0b00110010 0b00010000
		},
		valid: true,
	}, {
		input: `<<< < 11111111011011100101110101001100001110110010101000011001000010000`,
		valid: false,
	}, {
		input: `>>> < D0:0`,
		valid: true,
	}, {
		input: `>>> < D63:18364758544493064720`,
		valid: false,
	}, {
		input: `>>> > D63:18364758544493064720`,
		valid: false,
	}, {
		input:  `<<< < 01101 D11:1337 > 1011`,
		output: []byte{0x2d, 0xa7, 0x0d}, // 0b00101101 0b10100111 0b00001101
		valid:  true,
	}, {
		input: `
			<<<   	 # Comment 10110 < > D3:3

			# Comment
			> 01101 D11:1337    # Comment
			< 01101 D11:1337  # Comment > 010101

			X:abcdef

			D0:0 D0:0 D0:0
			# EOF
		`,
		output: []byte{
			0xb6, 0x9c, 0x2d, 0xa7, // 0b10110110 0b10011100 0b00101101 0b10100111
			0xab, 0xcd, 0xef,
		},
		valid: true,
	}, {
		input: `<<<
			< >01101 >D11:1337
			> <01101 <D11:1337
			  01101 D11:1337
			< 01101 D11:1337
			X:abcdef01
			> X:abcdef01
			< X:abcdef01
		`,
		output: []byte{
			0xb6, 0x9c, 0x2d, 0xa7, // 0b10110110 0b10011100 0b00101101 0b10100111
			0xb6, 0x9c, 0x2d, 0xa7, // 0b10110110 0b10011100 0b00101101 0b10100111
			0xab, 0xcd, 0xef, 0x01,
			0xab, 0xcd, 0xef, 0x01,
			0xab, 0xcd, 0xef, 0x01,
		},
		valid: true,
	}, {
		input: `>>>
			< >01101 >D11:1337
			> <01101 <D11:1337
			  01101 D11:1337
			< 01101 D11:1337
			X:abcdef01
			> X:abcdef01
			< X:abcdef01
		`,
		output: []byte{
			0x6d, 0x39, 0xb4, 0xe5, // 0b01101101 0b00111001 0b10110100 0b11100101
			0x6d, 0x39, 0xb4, 0xe5, // 0b01101101 0b00111001 0b10110100 0b11100101
			0xab, 0xcd, 0xef, 0x01,
			0xab, 0xcd, 0xef, 0x01,
			0xab, 0xcd, 0xef, 0x01,
		},
		valid: true,
	}, {
		input: `<<< < D12:1337*4 10101111*2 X:aBcD*3`,
		output: []byte{
			0x39, 0x95, 0x53, 0x39, 0x95, 0x53,
			0xaf, 0xaf, // 0b10101111 0b10101111
			0xab, 0xcd, 0xab, 0xcd, 0xab, 0xcd,
		},
		valid: true,
	}, {
		input: `<<< < D12:1337*0 10101111*0 X:aBcD*0`,
		valid: true,
	}, {
		input: `<<< < D12:1337*9999999999999999999999999999999999999999999999`,
		valid: false,
	}, {
		input: "<<< <X:abcd",
		valid: false,
	}, {
		input:  `<<< X:abcd < "The " "quick "*5 "brown \"fox\"\n\n \\njumped" > "" # HA`,
		output: []byte("\xab\xcdThe quick quick quick quick quick brown \"fox\"\n\n \\njumped"),
		valid:  true,
	}, {
		input:  `<<< ((("a")*2 "b")*2 "c")*2`,
		output: []byte("aabaabcaabaabc"),
		valid:  true,
	}, {
		input: `<<< (((("a")*2 "b")*2 "c")*2)*0 # Nothing`,
		valid: true,
	}, {
		input: `<<< (((()))`,
		valid: false,
	}, {
		input:  `<<< ((<()*5 ("hello")*2 ((<(<)) >)*123) "goodbye" ())*3`,
		output: []byte("hellohellogoodbyehellohellogoodbyehellohellogoodbye"),
		valid:  true,
	}, {
		input:  `>>> (("hello" <1110101 >D9:381)*2)`,
		output: []byte("hello\xaf}hello\xaf}"),
		valid:  true,
	}, {
		input:  `<<< > (1011 <(1011 1011 (< 1011 1011) 1011 (<1011) >(1011) (1011)) 1011)`,
		output: []byte{0xbd, 0xbb, 0xbb, 0xdb, 0xdb},
		valid:  true,
	}, {
		input:  `<<< >1011 <1011 <1011 <1011 <1011 <1011 <1011 >1011 <1011 >1011`,
		output: []byte{0xbd, 0xbb, 0xbb, 0xdb, 0xdb},
		valid:  true,
	}}

	for i, v := range vectors {
		output, err := DecodeBitGen(v.input)
		if (err == nil) != v.valid {
			if err != nil {
				t.Errorf("test %d, unexpected error: %v", i, err)
			} else {
				t.Errorf("test %d, unexpected success", i)
			}
			continue
		}
		if got, want, ok := BytesCompare(output, v.output); !ok {
			t.Errorf("test %d, mismatching output:\ngot  %s\nwant %s", i, got, want)
		}
	}
}
