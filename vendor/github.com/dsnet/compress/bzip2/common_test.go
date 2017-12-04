// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"strconv"
	"testing"

	"github.com/dsnet/compress/internal/testutil"
)

func TestCRC(t *testing.T) {
	vectors := []struct {
		crc uint32
		str string
	}{
		{0x00000000, ""},
		{0x19939b6b, "a"},
		{0xe993fdcd, "ab"},
		{0x648cbb73, "abc"},
		{0x3d4c334b, "abcd"},
		{0xa35b4df4, "abcde"},
		{0xa0f54fb9, "abcdef"},
		{0x077539d7, "abcdefg"},
		{0x5024ec61, "abcdefgh"},
		{0x63e0bcd4, "abcdefghi"},
		{0x73826444, "abcdefghij"},
		{0xbf786ee7, "Discard medicine more than two years old."},
		{0x106324f0, "He who has a shady past knows that nice guys finish last."},
		{0x0ef9b7d7, "I wouldn't marry him with a ten foot pole."},
		{0x2f42217b, "Free! Free!/A trip/to Mars/for 900/empty jars/Burma Shave"},
		{0xb64c598c, "The days of the digital watch are numbered.  -Tom Stoppard"},
		{0xf4e5a7c3, "Nepal premier won't resign."},
		{0x2b43233e, "For every action there is an equal and opposite government program."},
		{0x7b83ef6f, "His money is twice tainted: 'taint yours and 'taint mine."},
		{0x503c2258, "There is no reason for any individual to have a computer in their home. -Ken Olsen, 1977"},
		{0x4dc300fa, "It's a tiny change to the code and not completely disgusting. - Bob Manchek"},
		{0x97fa4243, "size:  a.out:  bad magic"},
		{0xc9549847, "The major problem is with sendmail.  -Mark Horton"},
		{0xeaa630ab, "Give me a rock, paper and scissors and I will move the world.  CCFestoon"},
		{0xcd8bb88c, "If the enemy is within range, then so are you."},
		{0x95cc0d9d, "It's well we cannot hear the screams/That we create in others' dreams."},
		{0x14c42897, "You remind me of a TV show, but that's all right: I watch it anyway."},
		{0x0de498f1, "C is as portable as Stonehedge!!"},
		{0x79e7cf74, "Even if I could be Shakespeare, I think I should still choose to be Faraday. - A. Huxley"},
		{0x33e2329e, "The fugacity of a constituent in a mixture of gases at a given temperature is proportional to its mole fraction.  Lewis-Randall Rule"},
		{0xa4302570, "How can you write a big system without C++?  -Paul Glick"},
	}

	var crc crc
	for i, v := range vectors {
		splits := []int{
			0 * (len(v.str) / 1),
			1 * (len(v.str) / 4),
			2 * (len(v.str) / 4),
			3 * (len(v.str) / 4),
			1 * (len(v.str) / 1),
		}
		for _, j := range splits {
			str1, str2 := []byte(v.str[:j]), []byte(v.str[j:])
			crc.val = 0
			crc.update(str1)
			if crc.update(str2); crc.val != v.crc {
				t.Errorf("test %d, crc.update(crc1, str2): got 0x%08x, want 0x%08x", i, crc.val, v.crc)
			}
		}
	}
}

func BenchmarkCRC(b *testing.B) {
	var c crc
	d := testutil.ResizeData([]byte("the quick brown fox jumped over the lazy dog"), 1<<16)
	for i := 1; i <= len(d); i <<= 4 {
		b.Run(strconv.Itoa(i), func(b *testing.B) {
			b.SetBytes(int64(i))
			for j := 0; j < b.N; j++ {
				c.update(d[:i])
			}
		})
	}
}
