// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"archive/tar"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/build/tarutil"
)

// put a .tar.gz
func putTar(args []string) error {
	fs := flag.NewFlagSet("put", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "puttar usage: gomote puttar [put-opts] <buildlet-name> [tar.gz file or '-' for stdin]")
		fs.PrintDefaults()
		os.Exit(1)
	}
	var rev string
	fs.StringVar(&rev, "gorev", "", "If non-empty, git hash to download from gerrit and put to the buildlet. e.g. 886b02d705ff for Go 1.4.1. This just maps to the --URL flag, so the two options are mutually exclusive.")
	var dir string
	fs.StringVar(&dir, "dir", "", "relative directory from buildlet's work dir to extra tarball into")
	var tarURL string
	fs.StringVar(&tarURL, "url", "", "URL of tarball, instead of provided file.")

	fs.Parse(args)
	if fs.NArg() < 1 || fs.NArg() > 2 {
		fs.Usage()
	}
	if rev != "" {
		if tarURL != "" {
			fmt.Fprintln(os.Stderr, "--gorev and --url are mutually exclusive")
			fs.Usage()
		}
		tarURL = "https://go.googlesource.com/go/+archive/" + rev + ".tar.gz"
	}

	name := fs.Arg(0)
	bc, _, err := clientAndConf(name)
	if err != nil {
		return err
	}

	if tarURL != "" {
		if fs.NArg() != 1 {
			fs.Usage()
		}
		if err := bc.PutTarFromURL(tarURL, dir); err != nil {
			return err
		}
		if rev != "" {
			// Put a VERSION file there too, to avoid git usage.
			version := strings.NewReader("devel " + rev)
			var vtar tarutil.FileList
			vtar.AddRegular(&tar.Header{
				Name: "VERSION",
				Mode: 0644,
				Size: int64(version.Len()),
			}, int64(version.Len()), version)
			tgz := vtar.TarGz()
			defer tgz.Close()
			return bc.PutTar(tgz, dir)
		}
		return nil
	}

	var tgz io.Reader = os.Stdin
	if fs.NArg() == 2 && fs.Arg(1) != "-" {
		f, err := os.Open(fs.Arg(1))
		if err != nil {
			return err
		}
		defer f.Close()
		tgz = f
	}
	return bc.PutTar(tgz, dir)
}

// put go1.4 in the workdir
func put14(args []string) error {
	fs := flag.NewFlagSet("put14", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "put14 usage: gomote put14 <buildlet-name>")
		fs.PrintDefaults()
		os.Exit(1)
	}
	fs.Parse(args)
	if fs.NArg() != 1 {
		fs.Usage()
	}
	name := fs.Arg(0)
	bc, conf, err := clientAndConf(name)
	if err != nil {
		return err
	}
	u := conf.GoBootstrapURL(buildEnv)
	if u == "" {
		fmt.Printf("No GoBootstrapURL defined for %q; ignoring. (may be baked into image)\n", name)
		return nil
	}
	return bc.PutTarFromURL(u, "go1.4")
}

// put single file
func put(args []string) error {
	fs := flag.NewFlagSet("put", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "put usage: gomote put [put-opts] <buildlet-name> <source or '-' for stdin> [destination]")
		fs.PrintDefaults()
		os.Exit(1)
	}
	modeStr := fs.String("mode", "", "Unix file mode (octal); default to source file mode")
	fs.Parse(args)
	if n := fs.NArg(); n < 2 || n > 3 {
		fs.Usage()
	}

	bc, _, err := clientAndConf(fs.Arg(0))
	if err != nil {
		return err
	}

	var r io.Reader = os.Stdin
	var mode os.FileMode = 0666

	src := fs.Arg(1)
	if src != "-" {
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		defer f.Close()
		r = f

		if *modeStr == "" {
			fi, err := f.Stat()
			if err != nil {
				return err
			}
			mode = fi.Mode()
		}
	}
	if *modeStr != "" {
		modeInt, err := strconv.ParseInt(*modeStr, 8, 64)
		if err != nil {
			return err
		}
		mode = os.FileMode(modeInt)
		if !mode.IsRegular() {
			return fmt.Errorf("bad mode: %v", mode)
		}
	}

	dest := fs.Arg(2)
	if dest == "" {
		if src == "-" {
			return errors.New("must specify destination file name when source is standard input")
		}
		dest = filepath.Base(src)
	}

	return bc.Put(r, dest, mode)
}
