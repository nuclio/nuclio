// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package loghash provides the shared information for computing
// a log hash (as in https://build.golang.org/log/HASH).
package loghash

import (
	"crypto/sha1"
	"fmt"
	"io"
)

// New returns a new hash for the given log text.
func New(text string) (hash string) {
	h := sha1.New()
	io.WriteString(h, text)
	return fmt.Sprintf("%x", h.Sum(nil))
}
