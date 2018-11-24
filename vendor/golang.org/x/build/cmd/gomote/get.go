// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
)

// get a .tar.gz
func getTar(args []string) error {
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "gettar usage: gomote gettar [get-opts] <buildlet-name>")
		fs.PrintDefaults()
		os.Exit(1)
	}
	var dir string
	fs.StringVar(&dir, "dir", "", "relative directory from buildlet's work dir to tar up")

	fs.Parse(args)
	if fs.NArg() != 1 {
		fs.Usage()
	}

	name := fs.Arg(0)
	bc, _, err := clientAndConf(name)
	if err != nil {
		return err
	}
	tgz, err := bc.GetTar(context.Background(), dir)
	if err != nil {
		return err
	}
	defer tgz.Close()
	_, err = io.Copy(os.Stdout, tgz)
	return err
}
