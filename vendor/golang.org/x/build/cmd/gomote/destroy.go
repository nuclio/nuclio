// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
)

func destroy(args []string) error {
	fs := flag.NewFlagSet("destroy", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "destroy usage: gomote destroy <instance>")
		fs.PrintDefaults()
		os.Exit(1)
	}

	fs.Parse(args)
	if fs.NArg() != 1 {
		fs.Usage()
	}
	name := fs.Arg(0)
	bc, _, err := clientAndConf(name)
	if err != nil {
		return err
	}
	return bc.Close()
}
