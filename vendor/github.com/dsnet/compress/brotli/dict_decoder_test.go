// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

import (
	"bytes"
	"strings"
	"testing"
)

func TestDictDecoder(t *testing.T) {
	const abc = "ABC\n"
	const fox = "The quick brown fox jumped over the lazy dog!\n"
	const poem = "The Road Not Taken\nRobert Frost\n" +
		"\n" +
		"Two roads diverged in a yellow wood,\n" +
		"And sorry I could not travel both\n" +
		"And be one traveler, long I stood\n" +
		"And looked down one as far as I could\n" +
		"To where it bent in the undergrowth;\n" +
		"\n" +
		"Then took the other, as just as fair,\n" +
		"And having perhaps the better claim,\n" +
		"Because it was grassy and wanted wear;\n" +
		"Though as for that the passing there\n" +
		"Had worn them really about the same,\n" +
		"\n" +
		"And both that morning equally lay\n" +
		"In leaves no step had trodden black.\n" +
		"Oh, I kept the first for another day!\n" +
		"Yet knowing how way leads on to way,\n" +
		"I doubted if I should ever come back.\n" +
		"\n" +
		"I shall be telling this with a sigh\n" +
		"Somewhere ages and ages hence:\n" +
		"Two roads diverged in a wood, and I-\n" +
		"I took the one less traveled by,\n" +
		"And that has made all the difference.\n"
	var refs = []struct {
		dist   int // Backward distance (0 if this is an insertion)
		length int // Length of copy or insertion
	}{
		{0, 38}, {33, 3}, {0, 48}, {79, 3}, {0, 11}, {34, 5}, {0, 6}, {23, 7},
		{0, 8}, {50, 3}, {0, 2}, {69, 3}, {34, 5}, {0, 4}, {97, 3}, {0, 4},
		{43, 5}, {0, 6}, {7, 4}, {88, 7}, {0, 12}, {80, 3}, {0, 2}, {141, 4},
		{0, 1}, {196, 3}, {0, 3}, {157, 3}, {0, 6}, {181, 3}, {0, 2}, {23, 3},
		{77, 3}, {28, 5}, {128, 3}, {110, 4}, {70, 3}, {0, 4}, {85, 6}, {0, 2},
		{182, 6}, {0, 4}, {133, 3}, {0, 7}, {47, 5}, {0, 20}, {112, 5}, {0, 1},
		{58, 3}, {0, 8}, {59, 3}, {0, 4}, {173, 3}, {0, 5}, {114, 3}, {0, 4},
		{92, 5}, {0, 2}, {71, 3}, {0, 2}, {76, 5}, {0, 1}, {46, 3}, {96, 4},
		{130, 4}, {0, 3}, {360, 3}, {0, 3}, {178, 5}, {0, 7}, {75, 3}, {0, 3},
		{45, 6}, {0, 6}, {299, 6}, {180, 3}, {70, 6}, {0, 1}, {48, 3}, {66, 4},
		{0, 3}, {47, 5}, {0, 9}, {325, 3}, {0, 1}, {359, 3}, {318, 3}, {0, 2},
		{199, 3}, {0, 1}, {344, 3}, {0, 3}, {248, 3}, {0, 10}, {310, 3}, {0, 3},
		{93, 6}, {0, 3}, {252, 3}, {157, 4}, {0, 2}, {273, 5}, {0, 14}, {99, 4},
		{0, 1}, {464, 4}, {0, 2}, {92, 4}, {495, 3}, {0, 1}, {322, 4}, {16, 4},
		{0, 3}, {402, 3}, {0, 2}, {237, 4}, {0, 2}, {432, 4}, {0, 1}, {483, 5},
		{0, 2}, {294, 4}, {0, 2}, {306, 3}, {113, 5}, {0, 1}, {26, 4}, {164, 3},
		{488, 4}, {0, 1}, {542, 3}, {248, 6}, {0, 5}, {205, 3}, {0, 8}, {48, 3},
		{449, 6}, {0, 2}, {192, 3}, {328, 4}, {9, 5}, {433, 3}, {0, 3}, {622, 25},
		{615, 5}, {46, 5}, {0, 2}, {104, 3}, {475, 10}, {549, 3}, {0, 4}, {597, 8},
		{314, 3}, {0, 1}, {473, 6}, {317, 5}, {0, 1}, {400, 3}, {0, 3}, {109, 3},
		{151, 3}, {48, 4}, {0, 4}, {125, 3}, {108, 3}, {0, 2},
	}

	var want string
	var buf bytes.Buffer
	var dd dictDecoder
	dd.Init(1 << 11)

	checkLastBytes := func(str string) {
		if len(str) < 2 {
			str = "\x00\x00" + str
		}
		str = str[len(str)-2:]
		p1, p2 := dd.LastBytes()
		got := string([]byte{p2, p1})
		if got != str {
			t.Errorf("last bytes mismatch: got %q, want %q", got, str)
		}
	}
	writeCopy := func(dist, length int) {
		if dist < length {
			cnt := (dist + length - 1) / dist
			want += strings.Repeat(want[len(want)-dist:], cnt)[:length]
		} else {
			want += want[len(want)-dist:][:length]
		}

		for length > 0 {
			length -= dd.WriteCopy(dist, length)
			if dd.AvailSize() == 0 {
				buf.Write(dd.ReadFlush())
			}
		}

		checkLastBytes(want)
	}
	writeString := func(str string) {
		want += str

		for len(str) > 0 {
			cnt := copy(dd.WriteSlice(), str)
			str = str[cnt:]
			dd.WriteMark(cnt)
			if dd.AvailSize() == 0 {
				buf.Write(dd.ReadFlush())
			}
		}

		checkLastBytes(want)
	}

	writeString("")
	writeString(".")
	str := poem
	for _, ref := range refs {
		if ref.dist == 0 {
			writeString(str[:ref.length])
		} else {
			writeCopy(ref.dist, ref.length)
		}
		str = str[ref.length:]
	}
	writeCopy(dd.HistSize(), 33)
	writeString(abc)
	writeCopy(len(abc), 59*len(abc))
	writeString(fox)
	writeCopy(len(fox), 9*len(fox))
	writeString(".")
	writeCopy(1, 9)
	writeString(strings.ToUpper(poem))
	writeCopy(len(poem), 7*len(poem))
	writeCopy(dd.HistSize(), 10)

	buf.Write(dd.ReadFlush())
	if buf.String() != want {
		t.Errorf("final string mismatch:\ngot  %q\nwant %q", buf.String(), want)
	}
}

func BenchmarkDictDecoderCopy(b *testing.B) {
	nb := 1 << 24
	b.SetBytes(int64(nb))

	for i := 0; i < b.N; i++ {
		var dd dictDecoder
		dd.Init(1 << 16)

		copy(dd.WriteSlice(), "abc")
		dd.WriteMark(3)

		dist, length := 3, nb
		for length > 0 {
			length -= dd.WriteCopy(dist, length)
			if dd.AvailSize() == 0 {
				dd.ReadFlush()
			}
		}
	}
}
