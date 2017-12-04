// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

// RFC section 8.
// Maximum buffer size needed to store a word after a transformation.
const maxWordSize = maxDictLen + 13 + 1

// These constants are defined in Appendix B of the RFC.
const (
	transformIdentity = iota
	transformUppercaseFirst
	transformUppercaseAll
	transformOmitFirst1
	transformOmitFirst2
	transformOmitFirst3
	transformOmitFirst4
	transformOmitFirst5
	transformOmitFirst6
	transformOmitFirst7
	transformOmitFirst8
	transformOmitFirst9
	transformOmitLast1
	transformOmitLast2
	transformOmitLast3
	transformOmitLast4
	transformOmitLast5
	transformOmitLast6
	transformOmitLast7
	transformOmitLast8
	transformOmitLast9
)

// This table is defined in Appendix B of the RFC.
var transformLUT = []struct {
	prefix    string
	transform int
	suffix    string
}{
	{"", transformIdentity, ""}, // 0
	{"", transformIdentity, " "},
	{" ", transformIdentity, " "},
	{"", transformOmitFirst1, ""},
	{"", transformUppercaseFirst, " "},
	{"", transformIdentity, " the "},
	{" ", transformIdentity, ""},
	{"s ", transformIdentity, " "},
	{"", transformIdentity, " of "},
	{"", transformUppercaseFirst, ""},
	{"", transformIdentity, " and "}, // 10
	{"", transformOmitFirst2, ""},
	{"", transformOmitLast1, ""},
	{", ", transformIdentity, " "},
	{"", transformIdentity, ", "},
	{" ", transformUppercaseFirst, " "},
	{"", transformIdentity, " in "},
	{"", transformIdentity, " to "},
	{"e ", transformIdentity, " "},
	{"", transformIdentity, "\""},
	{"", transformIdentity, "."}, // 20
	{"", transformIdentity, "\">"},
	{"", transformIdentity, "\n"},
	{"", transformOmitLast3, ""},
	{"", transformIdentity, "]"},
	{"", transformIdentity, " for "},
	{"", transformOmitFirst3, ""},
	{"", transformOmitLast2, ""},
	{"", transformIdentity, " a "},
	{"", transformIdentity, " that "},
	{" ", transformUppercaseFirst, ""}, // 30
	{"", transformIdentity, ". "},
	{".", transformIdentity, ""},
	{" ", transformIdentity, ", "},
	{"", transformOmitFirst4, ""},
	{"", transformIdentity, " with "},
	{"", transformIdentity, "'"},
	{"", transformIdentity, " from "},
	{"", transformIdentity, " by "},
	{"", transformOmitFirst5, ""},
	{"", transformOmitFirst6, ""}, // 40
	{" the ", transformIdentity, ""},
	{"", transformOmitLast4, ""},
	{"", transformIdentity, ". The "},
	{"", transformUppercaseAll, ""},
	{"", transformIdentity, " on "},
	{"", transformIdentity, " as "},
	{"", transformIdentity, " is "},
	{"", transformOmitLast7, ""},
	{"", transformOmitLast1, "ing "},
	{"", transformIdentity, "\n\t"}, // 50
	{"", transformIdentity, ":"},
	{" ", transformIdentity, ". "},
	{"", transformIdentity, "ed "},
	{"", transformOmitFirst9, ""},
	{"", transformOmitFirst7, ""},
	{"", transformOmitLast6, ""},
	{"", transformIdentity, "("},
	{"", transformUppercaseFirst, ", "},
	{"", transformOmitLast8, ""},
	{"", transformIdentity, " at "}, // 60
	{"", transformIdentity, "ly "},
	{" the ", transformIdentity, " of "},
	{"", transformOmitLast5, ""},
	{"", transformOmitLast9, ""},
	{" ", transformUppercaseFirst, ", "},
	{"", transformUppercaseFirst, "\""},
	{".", transformIdentity, "("},
	{"", transformUppercaseAll, " "},
	{"", transformUppercaseFirst, "\">"},
	{"", transformIdentity, "=\""}, // 70
	{" ", transformIdentity, "."},
	{".com/", transformIdentity, ""},
	{" the ", transformIdentity, " of the "},
	{"", transformUppercaseFirst, "'"},
	{"", transformIdentity, ". This "},
	{"", transformIdentity, ","},
	{".", transformIdentity, " "},
	{"", transformUppercaseFirst, "("},
	{"", transformUppercaseFirst, "."},
	{"", transformIdentity, " not "}, // 80
	{" ", transformIdentity, "=\""},
	{"", transformIdentity, "er "},
	{" ", transformUppercaseAll, " "},
	{"", transformIdentity, "al "},
	{" ", transformUppercaseAll, ""},
	{"", transformIdentity, "='"},
	{"", transformUppercaseAll, "\""},
	{"", transformUppercaseFirst, ". "},
	{" ", transformIdentity, "("},
	{"", transformIdentity, "ful "}, // 90
	{" ", transformUppercaseFirst, ". "},
	{"", transformIdentity, "ive "},
	{"", transformIdentity, "less "},
	{"", transformUppercaseAll, "'"},
	{"", transformIdentity, "est "},
	{" ", transformUppercaseFirst, "."},
	{"", transformUppercaseAll, "\">"},
	{" ", transformIdentity, "='"},
	{"", transformUppercaseFirst, ","},
	{"", transformIdentity, "ize "}, // 100
	{"", transformUppercaseAll, "."},
	{"\xc2\xa0", transformIdentity, ""},
	{" ", transformIdentity, ","},
	{"", transformUppercaseFirst, "=\""},
	{"", transformUppercaseAll, "=\""},
	{"", transformIdentity, "ous "},
	{"", transformUppercaseAll, ", "},
	{"", transformUppercaseFirst, "='"},
	{" ", transformUppercaseFirst, ","},
	{" ", transformUppercaseAll, "=\""}, // 110
	{" ", transformUppercaseAll, ", "},
	{"", transformUppercaseAll, ","},
	{"", transformUppercaseAll, "("},
	{"", transformUppercaseAll, ". "},
	{" ", transformUppercaseAll, "."},
	{"", transformUppercaseAll, "='"},
	{" ", transformUppercaseAll, ". "},
	{" ", transformUppercaseFirst, "=\""},
	{" ", transformUppercaseAll, "='"},
	{" ", transformUppercaseFirst, "='"}, // 120
}

// transformWord transform the input word and places the result in buf according
// to the transform primitives defined in RFC section 8.
//
// The following invariants must be kept:
//	0 <= id < len(transformLUT)
//	len(word) <= maxDictLen
//	len(buf) >= maxWordSize
func transformWord(buf, word []byte, id int) (cnt int) {
	transform := transformLUT[id]
	tid := transform.transform
	cnt = copy(buf, transform.prefix)
	switch {
	case tid == transformIdentity:
		cnt += copy(buf[cnt:], word)
	case tid == transformUppercaseFirst:
		buf2 := buf[cnt:]
		cnt += copy(buf2, word)
		transformUppercase(buf2[:len(word)], true)
	case tid == transformUppercaseAll:
		buf2 := buf[cnt:]
		cnt += copy(buf2, word)
		transformUppercase(buf2[:len(word)], false)
	case tid <= transformOmitFirst9:
		cut := tid - transformOmitFirst1 + 1 // 1..9
		if len(word) > cut {
			cnt += copy(buf[cnt:], word[cut:])
		}
	case tid <= transformOmitLast9:
		cut := tid - transformOmitLast1 + 1 // 1..9
		if len(word) > cut {
			cnt += copy(buf[cnt:], word[:len(word)-cut])
		}
	}
	cnt += copy(buf[cnt:], transform.suffix)
	return cnt
}

// transformUppercase transform the word to be in uppercase using the algorithm
// presented in RFC section 8. If once is set, then loop only executes once.
func transformUppercase(word []byte, once bool) {
	for i := 0; i < len(word); {
		c := word[i]
		if c < 192 {
			if c >= 97 && c <= 122 {
				word[i] ^= 32
			}
			i += 1
		} else if c < 224 {
			if i+1 < len(word) {
				word[i+1] ^= 32
			}
			i += 2
		} else {
			if i+2 < len(word) {
				word[i+2] ^= 5
			}
			i += 3
		}
		if once {
			return
		}
	}
}
