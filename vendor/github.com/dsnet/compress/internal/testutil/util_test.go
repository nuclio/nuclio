// Copyright 2017, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package testutil

import "testing"

func TestCompare(t *testing.T) {
	vectors := []struct {
		inA, inB   string
		outA, outB string
		ok         bool
	}{
		{"", "", "", "", true},
		{"", "foo", `""`, `"foo"`, false},
		{"bar", "foo", `"bar"`, `"foo"`, false},
		{"foo", "foo", "", "", true},
		{
			"keyboardsmashfoo", "keyboardsmashbar",
			`"keyboardsmashfoo"`, `"keyboardsmashbar"`,
			false,
		},
		{
			"keyboardsmashfoofjaewu893p4u4q893ru890q2urqr2r34r2fw42er32/q2890r3u0qv",
			"keyboardsmashfrioj8394ru4389",
			`"keyboardsmashfoofjaewu893p4u4q893ru890q2urqr2r34r2fw42er32/q2890"...(6 bytes)`,
			`"keyboardsmashfrioj8394ru4389"`,
			false,
		},
		{
			"keyboardsmashkeyboardsmashkeyboardsmashkeyboardsmashkeyboardsmashfoofjaewu893p4u4q893ru890q2urqr2r34r2fw42er3fefewaf",
			"keyboardsmashkeyboardsmashkeyboardsmashkeyboardsmashkeyboardsmashfrioj8394ru4389",
			`(16 bytes)..."boardsmashkeyboardsmashkeyboardsmashkeyboardsmashfoofjaewu893p4u"...(36 bytes)`,
			`(16 bytes)..."boardsmashkeyboardsmashkeyboardsmashkeyboardsmashfrioj8394ru4389"`,
			false,
		},
		{
			"keyboardsmashkeyboardsmashkeyboardsmashkeyboardsmashkeyboardsmashfoofjaewu893p4u4q893ru890q2urqr2r34r2fw42er3fefewaf",
			"keyboardsmashkeyboardsmashkeyboardsmashkeyboardsmashkeyboardsmashfriojfewafweafwaefweafewafwaefwaefwaefewafwae8394ru4389",
			`(34 bytes)..."smashkeyboardsmashkeyboardsmashfoofjaewu893p4u4q893ru890q2urqr2r"...(18 bytes)`,
			`(34 bytes)..."smashkeyboardsmashkeyboardsmashfriojfewafweafwaefweafewafwaefwae"...(22 bytes)`,
			false,
		},
		{
			"keyboardsmashkeyboardsmashkeyboardsmashkeyboardsmashkeyboardsmashfoofjaewu893p4u4q893ru890q2urqr2r34r2fw42er3fefewaf",
			"\xfaO\xed\x93QK\xb1\xa9O!\xc0\xac\x8dD\xd8\xce\xc01\x1aa\x9c\x108\xbb",
			`6b6579626f617264736d6173686b6579626f617264736d6173686b6579626f61...(84 bytes)`,
			`fa4fed93514bb1a94f21c0ac8d44d8cec0311a619c1038bb`,
			false,
		},
		{
			"keyboardsmashkeyboardsmashkeyboardsmashkeyboardsmashkeyboardsmashfoofjaewu893p4u4q893ru890q2urqr2r34r2fw42er3fefewaf",
			"keyboardsmashkeyboardsmashkeyboard\xfaO\xed\x93QK\xb1\xa9O!\xc0\xac\x8dD\xd8\xce\xc01\x1aa\x9c\x108\xbb",
			`(18 bytes)...617264736d6173686b6579626f617264736d6173686b6579626f617264736d61...(66 bytes)`,
			`(18 bytes)...617264736d6173686b6579626f617264fa4fed93514bb1a94f21c0ac8d44d8ce...(8 bytes)`,
			false,
		},
	}

	for i, v := range vectors {
		sa, sb, ok := BytesCompare([]byte(v.inA), []byte(v.inB))
		if sa != v.outA {
			t.Errorf("test %d, output A mismatch:\ngot  %s\nwant %s", i, sa, v.outA)
		}
		if sb != v.outB {
			t.Errorf("test %d, output B mismatch:\ngot  %s\nwant %s", i, sb, v.outB)
		}
		if ok != v.ok {
			t.Errorf("test %d, output equality mismatch: got %t, want %t", i, ok, v.ok)
		}
	}
}
