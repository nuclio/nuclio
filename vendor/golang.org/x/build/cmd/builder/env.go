// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// builderEnv represents the environment that a Builder will run tests in.
type builderEnv interface {
	// setup sets up the builder environment and returns the directory to run the buildCmd in.
	setup(repo *Repo, workpath, hash string, envv []string) (string, error)
}

// goEnv represents the builderEnv for the main Go repo.
type goEnv struct {
	goos, goarch string
}

func (b *Builder) crossCompile() bool {
	switch b.goos {
	case "android", "nacl":
		return true
	case "darwin":
		return b.goarch == "arm" || b.goarch == "arm64" // iOS
	default:
		return false
	}
}

func (b *Builder) envv() []string {
	if runtime.GOOS == "windows" {
		return b.envvWindows()
	}

	e := []string{
		"GOOS=" + b.goos,
		"GOARCH=" + b.goarch,
	}
	if !b.crossCompile() {
		// If we are building, for example, linux/386 on a linux/amd64 machine we want to
		// make sure that the whole build is done as a if this were compiled on a real
		// linux/386 machine. In other words, we want to not do a cross compilation build.
		// To do this we set GOHOSTOS and GOHOSTARCH to override the detection in make.bash.
		//
		// The exception to this rule is when we are doing nacl/android builds. These are by
		// definition always cross compilation, and we have support built into cmd/go to be
		// able to handle this case.
		e = append(e, "GOHOSTOS="+b.goos, "GOHOSTARCH="+b.goarch)
	}

	for _, k := range extraEnv() {
		if s, ok := getenvOk(k); ok {
			e = append(e, k+"="+s)
		}
	}
	return e
}

func (b *Builder) envvWindows() []string {
	start := map[string]string{
		"GOOS":        b.goos,
		"GOHOSTOS":    b.goos,
		"GOARCH":      b.goarch,
		"GOHOSTARCH":  b.goarch,
		"GOBUILDEXIT": "1", // exit all.bat with completion status.
	}

	for _, name := range extraEnv() {
		if s, ok := getenvOk(name); ok {
			start[name] = s
		}
	}
	if b.goos == "windows" {
		switch b.goarch {
		case "amd64":
			start["PATH"] = `c:\TDM-GCC-64\bin;` + start["PATH"]
		case "386":
			start["PATH"] = `c:\TDM-GCC-32\bin;` + start["PATH"]
		}
	}
	skip := map[string]bool{
		"GOBIN":   true,
		"GOPATH":  true,
		"GOROOT":  true,
		"INCLUDE": true,
		"LIB":     true,
	}
	var e []string
	for name, v := range start {
		e = append(e, name+"="+v)
		skip[name] = true
	}
	for _, kv := range os.Environ() {
		s := strings.SplitN(kv, "=", 2)
		name := strings.ToUpper(s[0])
		switch {
		case name == "":
			// variables, like "=C:=C:\", just copy them
			e = append(e, kv)
		case !skip[name]:
			e = append(e, kv)
			skip[name] = true
		}
	}
	return e
}

// setup for a goEnv clones the main go repo to workpath/go at the provided hash
// and returns the path workpath/go/src, the location of all go build scripts.
func (env *goEnv) setup(repo *Repo, workpath, hash string, envv []string) (string, error) {
	goworkpath := filepath.Join(workpath, "go")
	if err := repo.Export(goworkpath, hash); err != nil {
		return "", fmt.Errorf("error exporting repository: %s", err)
	}
	return filepath.Join(goworkpath, "src"), nil
}

func getenvOk(k string) (v string, ok bool) {
	v = os.Getenv(k)
	if v != "" {
		return v, true
	}
	keq := k + "="
	for _, kv := range os.Environ() {
		if kv == keq {
			return "", true
		}
	}
	return "", false
}

// extraEnv returns environment variables that need to be copied from
// the gobuilder's environment to the envv of its subprocesses.
func extraEnv() []string {
	extra := []string{
		"GOARM",
		"GO386",
		"GOROOT_BOOTSTRAP", // See https://golang.org/s/go15bootstrap
		"CGO_ENABLED",
		"CC",
		"CC_FOR_TARGET",
		"PATH",
		"TMPDIR",
		"USER",
		"HOME",
		"GO_TEST_TIMEOUT_SCALE", // increase test timeout for slow builders
	}
	if runtime.GOOS == "plan9" {
		extra = append(extra, "objtype", "cputype", "path")
	}
	return extra
}
