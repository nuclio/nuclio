// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package main

import "testing"

func TestIntFromStr(t *testing.T) {
	testcases := []struct {
		s string
		i int
	}{
		{"123", 123},
		{"User ID: 98403", 98403},
		{"1234 User 5431 ID", 1234},
		{"Stardate 153.2415", 153},
	}
	for _, tc := range testcases {
		r, ok := intFromStr(tc.s)
		if !ok {
			t.Errorf("intFromStr(%q) = %v", tc.s, ok)
		}
		if r != tc.i {
			t.Errorf("intFromStr(%q) = %d; want %d", tc.s, r, tc.i)
		}
	}
	noInt := "hello there"
	_, ok := intFromStr(noInt)
	if ok {
		t.Errorf("intFromStr(%q) = %v; want false", noInt, ok)
	}
}
