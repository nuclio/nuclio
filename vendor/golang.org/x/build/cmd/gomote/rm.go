// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
)

func rm(args []string) error {
	fs := flag.NewFlagSet("rm", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "rm usage: gomote rm <instance> <file-or-dir>+")
		fmt.Fprintln(os.Stderr, "          gomote rm <instance> .  (to delete everything)")
		fs.PrintDefaults()
		os.Exit(1)
	}
	fs.Parse(args)

	if fs.NArg() < 2 {
		fs.Usage()
	}
	name := fs.Arg(0)
	args = fs.Args()[1:]
	bc, _, err := clientAndConf(name)
	if err != nil {
		return err
	}
	return bc.RemoveAll(args...)
}
