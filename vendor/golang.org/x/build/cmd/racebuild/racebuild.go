// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// racebuild builds the race runtime (syso files) on all supported OSes using gomote.
// Usage:
//	$ racebuild -rev <llvm_git_revision> -goroot <path_to_go_repo>
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

var (
	flagGoroot    = flag.String("goroot", "", "path to Go repository to update (required)")
	flagRev       = flag.String("rev", "", "llvm compiler-rt git revision from http://llvm.org/git/compiler-rt.git (required)")
	flagGoRev     = flag.String("gorev", "HEAD", "Go repository revision to use; HEAD is relative to --goroot")
	flagPlatforms = flag.String("platforms", "all", `comma-separated platforms (such as "linux/amd64") to rebuild, or "all"`)
)

// goRev is the resolved commit ID of flagGoRev.
var goRev string

// TODO: use buildlet package instead of calling out to gomote.
var platforms = []*Platform{
	&Platform{
		OS:   "freebsd",
		Arch: "amd64",
		Type: "freebsd-amd64-race",
		Script: `#!/usr/bin/env bash
set -e
git clone https://go.googlesource.com/go
pushd go
git checkout $GOREV
popd
git clone http://llvm.org/git/compiler-rt.git
(cd compiler-rt && git checkout $REV)
(cd compiler-rt/lib/tsan/go && CC=clang ./buildgo.sh)
cp compiler-rt/lib/tsan/go/race_freebsd_amd64.syso go/src/runtime/race
(cd go/src && ./race.bash)
			`,
	},
	&Platform{
		OS:   "darwin",
		Arch: "amd64",
		Type: "darwin-amd64-10_12",
		Script: `#!/usr/bin/env bash
set -e
git clone https://go.googlesource.com/go
pushd go
git checkout $GOREV
popd
git clone http://llvm.org/git/compiler-rt.git
(cd compiler-rt && git checkout $REV)
(cd compiler-rt/lib/tsan/go && CC=clang ./buildgo.sh)
cp compiler-rt/lib/tsan/go/race_darwin_amd64.syso go/src/runtime/race
(cd go/src && ./race.bash)
			`,
	},
	&Platform{
		OS:   "linux",
		Arch: "amd64",
		Type: "linux-amd64-race",
		Script: `#!/usr/bin/env bash
set -e
apt-get update
apt-get install -y git g++
git clone https://go.googlesource.com/go
pushd go
git checkout $GOREV
popd
git clone http://llvm.org/git/compiler-rt.git
(cd compiler-rt && git checkout $REV)
(cd compiler-rt/lib/tsan/go && ./buildgo.sh)
cp compiler-rt/lib/tsan/go/race_linux_amd64.syso go/src/runtime/race
(cd go/src && ./race.bash)
			`,
	},
	&Platform{
		OS:   "linux",
		Arch: "ppc64le",
		Type: "linux-ppc64le-buildlet",
		Script: `#!/usr/bin/env bash
set -e
apt-get update
apt-get install -y git g++
git clone https://go.googlesource.com/go
pushd go
git checkout $GOREV
popd
git clone http://llvm.org/git/compiler-rt.git
(cd compiler-rt && git checkout $REV)
(cd compiler-rt/lib/tsan/go && ./buildgo.sh)
cp compiler-rt/lib/tsan/go/race_linux_ppc64le.syso go/src/runtime/race
# TODO(#23731): Uncomment to test the syso file before accepting it.
# (cd go/src && ./race.bash)
			`,
	},
	&Platform{
		OS:   "netbsd",
		Arch: "amd64",
		Type: "netbsd-amd64-8_0",
		Script: `#!/usr/bin/env bash
set -e
git clone https://go.googlesource.com/go
pushd go
git checkout $GOREV
popd
git clone http://llvm.org/git/compiler-rt.git
(cd compiler-rt && git checkout $REV)
(cd compiler-rt/lib/tsan/go && CC=clang ./buildgo.sh)
cp compiler-rt/lib/tsan/go/race_netbsd_amd64.syso go/src/runtime/race
# TODO(#24322): Uncomment to test the syso file before accepting it.
# (cd go/src && ./race.bash)
			`,
	},
	&Platform{
		OS:   "windows",
		Arch: "amd64",
		Type: "windows-amd64-race",
		Script: `
@"%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -InputFormat None -ExecutionPolicy Bypass -Command "iex ((New-Object System.Net.WebClient).DownloadString('https://chocolatey.org/install.ps1'))" && SET "PATH=%PATH%;%ALLUSERSPROFILE%\chocolatey\bin"
choco install git -y
if %errorlevel% neq 0 exit /b %errorlevel%
choco install mingw --version 5.3.0 -y
if %errorlevel% neq 0 exit /b %errorlevel%
call refreshenv
git clone https://go.googlesource.com/go
if %errorlevel% neq 0 exit /b %errorlevel%
cd go
git checkout %GOREV%
if %errorlevel% neq 0 exit /b %errorlevel%
cd ..
git clone http://llvm.org/git/compiler-rt.git
if %errorlevel% neq 0 exit /b %errorlevel%
cd compiler-rt
git checkout %REV%
if %errorlevel% neq 0 exit /b %errorlevel%
cd ..
cd compiler-rt/lib/tsan/go
call build.bat
if %errorlevel% neq 0 exit /b %errorlevel%
cd ../../../..
xcopy compiler-rt\lib\tsan\go\race_windows_amd64.syso go\src\runtime\race\race_windows_amd64.syso /Y
if %errorlevel% neq 0 exit /b %errorlevel%
cd go/src
call race.bat
if %errorlevel% neq 0 exit /b %errorlevel%
			`,
	},
}

func init() {
	// Ensure that there are no duplicate platform entries.
	seen := make(map[string]bool)
	for _, p := range platforms {
		if seen[p.Name()] {
			log.Fatalf("Duplicate platforms entry for %s.", p.Name())
		}
		seen[p.Name()] = true
	}
}

var platformEnabled = make(map[string]bool)

func parsePlatformsFlag() {
	if *flagPlatforms == "all" {
		for _, p := range platforms {
			platformEnabled[p.Name()] = true
		}
		return
	}

	var invalid []string
	for _, name := range strings.Split(*flagPlatforms, ",") {
		for _, p := range platforms {
			if name == p.Name() {
				platformEnabled[name] = true
				break
			}
		}
		if !platformEnabled[name] {
			invalid = append(invalid, name)
		}
	}

	if len(invalid) > 0 {
		var msg bytes.Buffer
		fmt.Fprintf(&msg, "Unrecognized platforms: %q. Supported platforms are:\n", invalid)
		for _, p := range platforms {
			fmt.Fprintf(&msg, "\t%s/%s\n", p.OS, p.Arch)
		}
		log.Fatal(&msg)
	}
}

func main() {
	flag.Parse()
	if *flagRev == "" || *flagGoroot == "" || *flagGoRev == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}
	parsePlatformsFlag()

	cmd := exec.Command("git", "rev-parse", *flagGoRev)
	cmd.Dir = *flagGoroot
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		log.Fatalf("%s failed: %v", strings.Join(cmd.Args, " "), err)
	}
	goRev = string(bytes.TrimSpace(out))
	log.Printf("using Go revision: %s", goRev)

	// Start build on all platforms in parallel.
	// On interrupt, destroy any in-flight builders before exiting.
	ctx, cancel := context.WithCancel(context.Background())
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt)
	go func() {
		<-shutdown
		cancel()
	}()

	g, ctx := errgroup.WithContext(ctx)
	for _, p := range platforms {
		if !platformEnabled[p.Name()] {
			continue
		}

		p := p
		g.Go(func() error {
			if err := p.Build(ctx); err != nil {
				return fmt.Errorf("%v failed: %v", p.Name(), err)
			}
			return p.UpdateReadme()
		})
	}

	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}
}

type Platform struct {
	OS     string
	Arch   string
	Type   string // gomote instance type
	Inst   string // actual gomote instance name
	Script string
}

func (p *Platform) Name() string {
	return fmt.Sprintf("%v/%v", p.OS, p.Arch)
}

// Basename returns the name of the output file relative to src/runtime/race.
func (p *Platform) Basename() string {
	return fmt.Sprintf("race_%v_%s.syso", p.OS, p.Arch)
}

func (p *Platform) Build(ctx context.Context) error {
	// Create gomote instance (or reuse an existing instance for debugging).
	var lastErr error
	for p.Inst == "" {
		inst, err := p.Gomote(ctx, "create", p.Type)
		if err != nil {
			select {
			case <-ctx.Done():
				if lastErr != nil {
					return lastErr
				}
				return err
			default:
				// Creation sometimes fails with transient errors like:
				// "buildlet didn't come up at http://10.240.0.13 in 3m0s".
				log.Printf("%v: instance creation failed, retrying", p.Name())
				lastErr = err
				continue
			}
		}
		p.Inst = strings.Trim(string(inst), " \t\n")
		defer p.Gomote(context.Background(), "destroy", p.Inst)
	}
	log.Printf("%s: using instance %v", p.Name(), p.Inst)

	// put14
	if _, err := p.Gomote(ctx, "put14", p.Inst); err != nil {
		return err
	}

	// Execute the script.
	script, err := ioutil.TempFile("", "racebuild")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer func() {
		script.Close()
		os.Remove(script.Name())
	}()
	if _, err := script.Write([]byte(p.Script)); err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}
	script.Close()
	targetName := "script.bash"
	if p.OS == "windows" {
		targetName = "script.bat"
	}
	if _, err := p.Gomote(ctx, "put", "-mode=0700", p.Inst, script.Name(), targetName); err != nil {
		return err
	}
	if _, err := p.Gomote(ctx, "run", "-e=REV="+*flagRev, "-e=GOREV="+goRev, p.Inst, targetName); err != nil {
		return err
	}

	// The script is supposed to leave updated runtime at that path. Copy it out.
	syso := p.Basename()
	targz, err := p.Gomote(ctx, "gettar", "-dir=go/src/runtime/race/"+syso, p.Inst)
	if err != nil {
		return err
	}

	// Untar the runtime and write it to goroot.
	if err := p.WriteSyso(filepath.Join(*flagGoroot, "src", "runtime", "race", syso), targz); err != nil {
		return fmt.Errorf("%v", err)
	}

	log.Printf("%v: build completed", p.Name())
	return nil
}

func (p *Platform) WriteSyso(sysof string, targz []byte) error {
	// Ungzip.
	gzipr, err := gzip.NewReader(bytes.NewReader(targz))
	if err != nil {
		return fmt.Errorf("failed to read gzip archive: %v", err)
	}
	defer gzipr.Close()
	tr := tar.NewReader(gzipr)
	if _, err := tr.Next(); err != nil {
		return fmt.Errorf("failed to read tar archive: %v", err)
	}

	// Copy the file.
	syso, err := os.Create(sysof)
	if err != nil {
		return fmt.Errorf("failed to open race runtime: %v", err)
	}
	defer syso.Close()
	if _, err := io.Copy(syso, tr); err != nil {
		return fmt.Errorf("failed to write race runtime: %v", err)
	}
	return nil
}

var readmeMu sync.Mutex

func (p *Platform) UpdateReadme() error {
	readmeMu.Lock()
	defer readmeMu.Unlock()

	readmeFile := filepath.Join(*flagGoroot, "src", "runtime", "race", "README")
	readme, err := ioutil.ReadFile(readmeFile)
	if err != nil {
		log.Fatalf("bad -goroot? %v", err)
	}

	syso := p.Basename()
	const (
		readmeTmpl = "%s built with LLVM %s and Go %s."
		commitRE   = "[0-9a-f]+"
	)

	// TODO(bcmills): Extract the C++ toolchain version from the .syso file and
	// record it in the README.
	updatedLine := fmt.Sprintf(readmeTmpl, syso, *flagRev, goRev)

	lineRE, err := regexp.Compile("(?m)^" + fmt.Sprintf(readmeTmpl, regexp.QuoteMeta(syso), commitRE, commitRE) + "$")
	if err != nil {
		return err
	}
	if lineRE.Match(readme) {
		readme = lineRE.ReplaceAll(readme, []byte(updatedLine))
	} else {
		readme = append(append(readme, []byte(updatedLine)...), '\n')
	}

	return ioutil.WriteFile(readmeFile, readme, 0640)
}

func (p *Platform) Gomote(ctx context.Context, args ...string) ([]byte, error) {
	log.Printf("%v: gomote %v", p.Name(), args)

	cmd := exec.CommandContext(ctx, "gomote", args...)
	outBuf := new(bytes.Buffer)

	// Combine stderr and stdout for everything except gettar: gettar's output is
	// huge, so we only want to log stderr for it.
	errBuf := outBuf
	if args[0] == "gettar" {
		errBuf = new(bytes.Buffer)
	}

	cmd.Stdout = outBuf
	cmd.Stderr = errBuf
	run := cmd.Run
	if len(platformEnabled) == 1 {
		// If building only one platform, stream gomote output to os.Stderr.
		r, w := io.Pipe()
		errTee := io.TeeReader(r, cmd.Stderr)
		if cmd.Stdout == cmd.Stderr {
			cmd.Stdout = w
		}
		cmd.Stderr = w

		run = func() (err error) {
			go func() {
				err = cmd.Run()
				w.Close()
			}()
			io.Copy(os.Stderr, errTee)
			return
		}
	}

	if err := run(); err != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		log.Printf("%v: gomote %v failed:\n%s", p.Name(), args, errBuf)
		return nil, err
	}

	if errBuf.Len() == 0 {
		log.Printf("%v: gomote %v succeeded: <no output>", p.Name(), args)
	} else {
		log.Printf("%v: gomote %v succeeded:\n%s", p.Name(), args, errBuf)
	}
	return outBuf.Bytes(), nil
}
