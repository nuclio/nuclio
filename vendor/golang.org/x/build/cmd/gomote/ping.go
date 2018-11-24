// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
)

func ping(args []string) error {
	fs := flag.NewFlagSet("ping", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "ping usage: gomote ping <instance>")
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
	_, err = bc.WorkDir()
	return err
}
