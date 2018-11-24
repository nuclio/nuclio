// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "testing"

func TestTitleDir(t *testing.T) {
	testcases := []struct {
		title string
		dirs  []string
	}{
		{"no title dir", nil},
		{"  cmd/compile   ,  cmd/go: do awesome things", []string{"cmd/compile", "cmd/go"}},
		{"cmd/compile: cleanup MOVaddr code generation", []string{"cmd/compile"}},
		{`cmd/asm, cmd/internal/obj/s390x, math: add "test under mask" instructions`,
			[]string{"cmd/asm", "cmd/internal/obj/s390x", "math"}},
	}
	for _, tc := range testcases {
		r := titleDirs(tc.title)
		if len(r) != len(tc.dirs) {
			t.Fatalf("titleDirs(%q) = %v (%d); want %d length", tc.title, r, len(r), len(tc.dirs))
		}
		for i := range tc.dirs {
			if r[i] != tc.dirs[i] {
				t.Errorf("titleDirs[%d](%v) != tc.dirs[%d](%q)", i, r[i], i, tc.dirs[i])
			}
		}
	}
}
