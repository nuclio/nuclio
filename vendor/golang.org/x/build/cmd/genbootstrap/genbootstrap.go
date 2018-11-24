// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
The genbootstrap command prepares GO_BOOTSTRAP tarballs suitable for
use on builders. It's a wrapper around bootstrap.bash. After
bootstrap.bash produces the full output, genbootstrap trims it up,
removing unnecessary and unwanted files.

Usage:  genbootstrap GOOS/GOARCH
*/
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var skipBuild = flag.Bool("skip_build", false, "skip bootstrap.bash step; useful during development of cleaning code")

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: genbootstrap GOOS/GOARCH")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	f := strings.Split(flag.Arg(0), "/")
	if len(f) != 2 {
		flag.Usage()
		os.Exit(2)
	}
	goos, goarch := f[0], f[1]
	if os.Getenv("GOROOT") == "" {
		log.Fatalf("GOROOT not set in environment")
	}

	tgz := filepath.Join(os.Getenv("GOROOT"), "src", "..", "..", "gobootstrap-"+goos+"-"+goarch+".tar.gz")
	os.Remove(tgz)
	outDir := filepath.Join(os.Getenv("GOROOT"), "src", "..", "..", "go-"+goos+"-"+goarch+"-bootstrap")
	if !*skipBuild {
		os.RemoveAll(outDir)
		cmd := exec.Command(filepath.Join(os.Getenv("GOROOT"), "src", "bootstrap.bash"))
		cmd.Dir = filepath.Join(os.Getenv("GOROOT"), "src")
		cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}

		// bootstrap.bash makes a bzipped tar file too, but it's fat and full of stuff we
		// dont need it. delete it.
		os.Remove(outDir + ".tbz")
	}

	if err := filepath.Walk(outDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(path, outDir), "/")
		base := filepath.Base(path)
		var pkgrel string // relative to pkg/<goos>_<goarch>/, or empty
		if strings.HasPrefix(rel, "pkg/") && strings.Count(rel, "/") >= 2 {
			pkgrel = strings.TrimPrefix(rel, "pkg/")
			pkgrel = pkgrel[strings.Index(pkgrel, "/")+1:]
			log.Printf("rel %q => %q", rel, pkgrel)
		}
		remove := func() error {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		switch pkgrel {
		case "cmd":
			return remove()
		}
		switch rel {
		case "api",
			"bin/gofmt",
			"doc",
			"misc/android",
			"misc/cgo",
			"misc/chrome",
			"misc/swig",
			"test":
			return remove()
		}
		if base == "testdata" {
			return remove()
		}
		if strings.HasPrefix(rel, "pkg/tool/") {
			switch base {
			case "addr2line", "api", "cgo", "cover",
				"dist", "doc", "fix", "nm",
				"objdump", "pack", "pprof",
				"trace", "vet", "yacc":
				return remove()
			}
		}
		if fi.IsDir() {
			return nil
		}
		if isEditorJunkFile(path) {
			return remove()
		}
		if !fi.Mode().IsRegular() {
			return remove()
		}
		if strings.HasSuffix(path, "_test.go") {
			return remove()
		}
		log.Printf("keeping: %s\n", rel)
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	log.Printf("Running: tar zcf %s .", tgz)
	cmd := exec.Command("tar", "zcf", tgz, ".")
	cmd.Dir = outDir
	if err := cmd.Run(); err != nil {
		log.Fatalf("tar zf failed: %v", err)
	}
	log.Printf("Done. Output is %s", tgz)
}

func isEditorJunkFile(path string) bool {
	path = filepath.Base(path)
	if strings.HasPrefix(path, "#") && strings.HasSuffix(path, "#") {
		return true
	}
	if strings.HasSuffix(path, "~") {
		return true
	}
	return false
}
