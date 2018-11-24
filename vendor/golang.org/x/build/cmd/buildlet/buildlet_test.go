// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"runtime"
	"testing"
)

func TestSetPathEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO(adg): make this test work on windows")
	}

	const workDir = "/workdir"

	for _, c := range []struct {
		env  []string
		path []string
		want []string
	}{
		{ // No change to PATH
			[]string{"A=1", "PATH=/bin:/usr/bin", "B=2"},
			[]string{},
			[]string{"A=1", "PATH=/bin:/usr/bin", "B=2"},
		},
		{ // Test sentinel $EMPTY value to clear PATH
			[]string{"A=1", "PATH=/bin:/usr/bin", "B=2"},
			[]string{"$EMPTY"},
			[]string{"A=1", "B=2"},
		},
		{ // Test $WORKDIR expansion
			[]string{"A=1", "PATH=/bin:/usr/bin", "B=2"},
			[]string{"/go/bin", "$WORKDIR/foo"},
			[]string{"A=1", "PATH=/go/bin:/workdir/foo", "B=2"},
		},
		{ // Test $PATH expansion
			[]string{"A=1", "PATH=/bin:/usr/bin", "B=2"},
			[]string{"/go/bin", "$PATH", "$WORKDIR/foo"},
			[]string{"A=1", "PATH=/go/bin:/bin:/usr/bin:/workdir/foo", "B=2"},
		},
		{ // Test $PATH expansion (prepend only)
			[]string{"A=1", "PATH=/bin:/usr/bin", "B=2"},
			[]string{"/go/bin", "/a/b", "$PATH"},
			[]string{"A=1", "PATH=/go/bin:/a/b:/bin:/usr/bin", "B=2"},
		},
		{ // Test $PATH expansion (append only)
			[]string{"A=1", "PATH=/bin:/usr/bin", "B=2"},
			[]string{"$PATH", "/go/bin", "/a/b"},
			[]string{"A=1", "PATH=/bin:/usr/bin:/go/bin:/a/b", "B=2"},
		},
	} {
		got := setPathEnv(c.env, c.path, workDir)
		if g, w := fmt.Sprint(got), fmt.Sprint(c.want); g != w {
			t.Errorf("setPathEnv(%q, %q) = %q, want %q", c.env, c.path, g, w)
		}
	}
}
