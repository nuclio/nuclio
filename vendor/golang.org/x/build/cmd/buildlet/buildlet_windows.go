// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"os"
	"syscall"
	"unsafe"

	"github.com/tarm/serial"
)

func init() {
	killProcessTree = killProcessTreeWindows
	configureSerialLogOutput = configureSerialLogOutputWindows
}

func configureSerialLogOutputWindows() {
	c := &serial.Config{Name: "COM1", Baud: 9600}
	s, err := serial.OpenPort(c)
	if err != nil {
		// Oh well, we tried. This empirically works
		// on Windows on GCE.
		// We can log here anyway and hope somebody sees it
		// in a GUI console:
		log.Printf("serial.OpenPort: %v", err)
		return
	}
	log.SetOutput(s)
}

// the system process tree
type psTree map[int]int // pid -> parent pid

// findDescendants searches process tree t for pid process children.
// It returns children pids.
func (t psTree) findDescendants(pid int) []int {
	var children []int
	for child, parent := range t {
		if parent == pid {
			children = append(children, child)
			children = append(children, t.findDescendants(child)...)
		}
	}
	return children
}

func snapshotSysProcesses() (psTree, error) {
	ss, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(ss)

	ps := make(psTree)

	var pe syscall.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err = syscall.Process32First(ss, &pe); err != nil {
		return nil, err
	}
	for {
		ps[int(pe.ProcessID)] = int(pe.ParentProcessID)
		err = syscall.Process32Next(ss, &pe)
		if err == syscall.ERROR_NO_MORE_FILES {
			return ps, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

func killProcesses(ps []int) {
	for _, pid := range ps {
		p, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		p.Kill()
		p.Release()
	}
}

func killProcessTreeWindows(p *os.Process) error {
	ps, err := snapshotSysProcesses()
	if err != nil {
		return err
	}
	toKill := ps.findDescendants(p.Pid)
	toKill = append(toKill, p.Pid)
	killProcesses(toKill)
	return nil
}
