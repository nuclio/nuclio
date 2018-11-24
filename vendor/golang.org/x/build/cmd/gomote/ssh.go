// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/build/internal/gophers"
)

func ssh(args []string) error {
	fs := flag.NewFlagSet("ssh", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "ssh usage: gomote ssh <instance>")
		fs.PrintDefaults()
		os.Exit(1)
	}
	var mutable bool
	fs.BoolVar(&mutable, "i-will-not-break-the-host", false, "required for older host configs with reused filesystems; using this says that you are aware that your changes to the machine's root filesystem affect future builds. This is a no-op for the newer, safe host configs.")
	fs.Parse(args)
	if fs.NArg() != 1 {
		fs.Usage()
	}
	name := fs.Arg(0)
	_, _, err := clientAndConf(name)
	if err != nil {
		return err
	}
	// gomoteUser extracts "gopher" from "user-gopher-linux-amd64-0".
	gomoteUser := strings.Split(name, "-")[1]
	githubUser := gophers.GithubOfGomoteUser(gomoteUser)

	sshUser := name
	if mutable {
		sshUser = "mutable-" + sshUser
	}

	ssh, err := exec.LookPath("ssh")
	if err != nil {
		log.Printf("No 'ssh' binary found in path so can't run:")
	}
	fmt.Printf("$ ssh -p 2222 %s@farmer.golang.org # auth using https://github.com/%s.keys\n", sshUser, githubUser)

	// Best effort, where supported:
	syscall.Exec(ssh, []string{"ssh", "-p", "2222", sshUser + "@farmer.golang.org"}, os.Environ())
	return nil
}
