// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// BitGen to generate a binary from a BitGen formatting input.
// It accepts the BitGen format from stdin and outputs to stdout.
package main

import (
	"io/ioutil"
	"os"

	"github.com/dsnet/compress/internal/testutil"
)

func main() {
	buf, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}

	buf = testutil.MustDecodeBitGen(string(buf))

	_, err = os.Stdout.Write(buf)
	if err != nil {
		panic(err)
	}
}
