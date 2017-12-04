// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

import (
	"bytes"
	"errors"
	"flag"
	"os/exec"
	"strings"
)

var zcheck = flag.Bool("zcheck", false, "verify test vectors with C brotli library")

func cmdCompress(input []byte) ([]byte, error)   { return cmdExec(input) }
func cmdDecompress(input []byte) ([]byte, error) { return cmdExec(input, "-d") }

// cmdExec executes the bzip2 tool, passing the input in as stdin.
// It returns the stdout and an error.
func cmdExec(input []byte, args ...string) ([]byte, error) {
	var bo, be bytes.Buffer
	cmd := exec.Command("bro", args...)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = &bo
	cmd.Stderr = &be
	err := cmd.Run()
	ss := strings.Split(strings.TrimSpace(be.String()), "\n")
	if len(ss) > 0 && ss[len(ss)-1] != "" {
		// Assume any stderr indicates an error and last line is the message.
		return nil, errors.New(ss[len(ss)-1])
	}
	return bo.Bytes(), err
}
