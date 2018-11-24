// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"golang.org/x/build/buildlet"
	"golang.org/x/build/dashboard"
)

func list(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "list usage: gomote list")
		fs.PrintDefaults()
		os.Exit(1)
	}
	fs.Parse(args)
	if fs.NArg() != 0 {
		fs.Usage()
	}

	cc, err := buildlet.NewCoordinatorClientFromFlags()
	if err != nil {
		log.Fatal(err)
	}
	rbs, err := cc.RemoteBuildlets()
	if err != nil {
		log.Fatal(err)
	}
	for _, rb := range rbs {
		fmt.Printf("%s\t%s\t%s\texpires in %v\n", rb.Name, rb.BuilderType, rb.HostType, rb.Expires.Sub(time.Now()))
	}

	return nil
}

// clientAndConfig returns a buildlet.Client and its build config for
// a named remote buildlet (a buildlet connection owned by the build
// coordinator).
//
// As a special case, if name contains '@', the name is expected to be
// of the form <build-config-name>@ip[:port]. For example,
// "windows-amd64-race@10.0.0.1".
func clientAndConf(name string) (bc *buildlet.Client, conf *dashboard.BuildConfig, err error) {
	var ok bool

	if strings.Contains(name, "@") {
		f := strings.SplitN(name, "@", 2)
		if len(f) != 2 {
			err = fmt.Errorf("unsupported name %q; for @ form expect <build-config-name>@host[:port]", name)
			return
		}
		builderType := f[0]
		conf, ok = dashboard.Builders[builderType]
		if !ok {
			err = fmt.Errorf("unknown builder type %q (name %q)", builderType, name)
			return
		}
		ipPort := f[1]
		if !strings.Contains(ipPort, ":") {
			ipPort += ":80"
		}
		bc = buildlet.NewClient(ipPort, buildlet.NoKeyPair)
		return
	}

	cc, err := buildlet.NewCoordinatorClientFromFlags()
	if err != nil {
		return
	}

	rbs, err := cc.RemoteBuildlets()
	if err != nil {
		return
	}
	for _, rb := range rbs {
		if rb.Name == name {
			conf, ok = dashboard.Builders[rb.BuilderType]
			if !ok {
				err = fmt.Errorf("builder %q exists, but unknown builder type %q", name, rb.BuilderType)
				return
			}
			break
		}
	}
	if !ok {
		err = fmt.Errorf("unknown builder %q", name)
		return
	}

	bc, err = cc.NamedBuildlet(name)
	if err != nil {
		return
	}
	return bc, conf, nil
}
