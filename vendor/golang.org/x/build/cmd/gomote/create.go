// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"golang.org/x/build/buildlet"
	"golang.org/x/build/dashboard"
)

func vmTypes() (s []string) {
	for k := range dashboard.Builders {
		s = append(s, k)
	}
	sort.Strings(s)
	return
}

func create(args []string) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "create usage: gomote create [create-opts] <type>")
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nValid types:")
		for _, t := range vmTypes() {
			fmt.Fprintf(os.Stderr, "  * %s\n", t)
		}
		os.Exit(1)
	}
	// TODO(bradfitz): restore this option, and send it to the coordinator:
	// For now, comment it out so it's not misleading.
	// var timeout time.Duration
	// fs.DurationVar(&timeout, "timeout", 60*time.Minute, "how long the VM will live before being deleted.")

	fs.Parse(args)
	if fs.NArg() != 1 {
		fs.Usage()
	}
	builderType := fs.Arg(0)
	_, ok := dashboard.Builders[builderType]
	if !ok {
		var valid []string
		var prefixMatch []string
		for k := range dashboard.Builders {
			valid = append(valid, k)
			if strings.HasPrefix(k, builderType) {
				prefixMatch = append(prefixMatch, k)
			}
		}
		if len(prefixMatch) == 1 {
			builderType = prefixMatch[0]
		} else {
			sort.Strings(valid)
			return fmt.Errorf("Invalid builder type %q. Valid options include: %q", builderType, valid)
		}
	}

	cc, err := buildlet.NewCoordinatorClientFromFlags()
	if err != nil {
		return fmt.Errorf("failed to create coordinator client: %v", err)
	}
	client, err := cc.CreateBuildlet(builderType)
	if err != nil {
		return fmt.Errorf("failed to create buildlet: %v", err)
	}
	fmt.Println(client.RemoteName())
	return nil
}
