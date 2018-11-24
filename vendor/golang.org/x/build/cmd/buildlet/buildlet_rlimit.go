// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build netbsd openbsd

package main

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

func init() {
	switch runtime.GOOS {
	case "netbsd":
		setOSRlimit = setNetBSDRlimit
	case "openbsd":
		setOSRlimit = setOpenBSDRlimit
	}
}

// setNetBSDRlimit sets limits for NetBSD.
// See https://github.com/golang/go/issues/22871#issuecomment-346888363
func setNetBSDRlimit() error {
	limit := unix.Rlimit{
		Cur: unix.RLIM_INFINITY,
		Max: unix.RLIM_INFINITY,
	}
	if err := unix.Setrlimit(unix.RLIMIT_DATA, &limit); err != nil && os.Getuid() == 0 {
		return err
	}
	return nil
}

// setOpenBSDRlimit sets limits for OpenBSD.
// See https://go-review.googlesource.com/c/go/+/81876
func setOpenBSDRlimit() error {
	var lim unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &lim); err != nil {
		return fmt.Errorf("getting initial rlimit: %v", err)
	}
	log.Printf("initial NOFILE rlimit: %+v", lim)

	lim.Cur = 32 << 10
	lim.Max = 32 << 10
	if err := unix.Setrlimit(unix.RLIMIT_NOFILE, &lim); err != nil && os.Getuid() == 0 {
		return fmt.Errorf("Setrlimit: %v", err)
	}

	if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &lim); err != nil {
		return fmt.Errorf("getting updated rlimit: %v", err)
	}
	log.Printf("updated NOFILE rlimit: %+v", lim)

	return nil
}
