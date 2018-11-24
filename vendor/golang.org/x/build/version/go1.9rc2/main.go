// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The go1.9rc2 command runs the go command from go1.9rc2.
//
// Deprecated: Use https://godoc.org/golang.org/dl/go1.9rc2 instead.
package main

import "golang.org/x/build/version"

func main() {
	version.Run("go1.9rc2")
}
