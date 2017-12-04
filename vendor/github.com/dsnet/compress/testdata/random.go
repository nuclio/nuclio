// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build ignore

// Generates random.bin. This test file contains random data throughout and
// tests the worst case compression scenario.
package main

import (
	"io/ioutil"
	"math/rand"
)

const (
	name = "random.bin"
	size = 1 << 18
)

func main() {
	var b []byte
	r := rand.New(rand.NewSource(0))

	for i := 0; i < size; i++ {
		b = append(b, byte(r.Int()))
	}
	if err := ioutil.WriteFile(name, b[:size], 0664); err != nil {
		panic(err)
	}
}
