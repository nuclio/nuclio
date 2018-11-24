// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux
// (We only care about Linux on GKE for now)

package main

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func init() {
	registerSignal = registerSignalUnix
	setWorkdirToTmpfs = setWorkdirToTmpfsLinux
}

func registerSignalUnix(c chan<- os.Signal) {
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
}

func setWorkdirToTmpfsLinux() {
	const dir = "/workdir"

	var st syscall.Statfs_t
	const TMPFS_MAGIC = 0x01021994 // from statfs(2)
	if syscall.Statfs(dir, &st) != nil || st.Type != TMPFS_MAGIC {
		return
	}
	log.Printf("detected tmpfs %s", dir)

	const ST_NOEXEC = syscall.MS_NOEXEC // golang.org/issue/25341
	if st.Flags&ST_NOEXEC != 0 {
		log.Printf("tmpfs %s is noexec; remounting", dir)
		out, err := exec.Command("mount", "-o", "remount,exec", dir).CombinedOutput()
		if err != nil {
			log.Printf("error remounting tmpfs %s as exec: %v, %s", dir, err, out)
			return
		}
		// Check that it worked:
		st = syscall.Statfs_t{}
		if err := syscall.Statfs(dir, &st); err != nil {
			log.Printf("second stat of %s failed after remount: %v", dir, err)
			return
		}
		if st.Flags&ST_NOEXEC != 0 {
			log.Printf("remount of %s failed; still marked noexec", dir)
			return
		}
		log.Printf("remounted tmpfs %s to remove noexec", dir)
	}
	*workDir = dir
	log.Printf("using tmpfs %s as workdir.", dir)
}
