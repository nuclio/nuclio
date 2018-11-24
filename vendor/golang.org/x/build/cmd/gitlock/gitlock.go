// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The gitlock command helps write Dockerfiles with a bunch of lines
// to lock git dependencies in place.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

var (
	ignorePrefixFlag = flag.String("ignore", "golang.org/x/build", "comma-separated list of package prefixes to ignore")
	updateFile       = flag.String("update", "", "if non-empty, the Dockerfile to update. must have \"# BEGIN deps\" and \"# END deps\" lines.")
	tags             = flag.String("tags", "", "space-separated tags to pass on to 'go list -tags=XXX target'")
)

var ignorePrefixes []string

func parseDockerFile() (header, footer []byte) {
	if *updateFile == "" {
		return nil, nil
	}
	var headBuf, footBuf bytes.Buffer
	slurp, err := ioutil.ReadFile(*updateFile)
	if err != nil {
		log.Fatal(err)
	}
	var sawBegin, sawEnd bool
	bs := bufio.NewScanner(bytes.NewReader(slurp))
	for bs.Scan() {
		if !sawBegin {
			headBuf.Write(bs.Bytes())
			headBuf.WriteByte('\n')
			if strings.HasPrefix(bs.Text(), "# BEGIN deps") {
				sawBegin = true
				continue
			}
			continue
		}
		if strings.HasPrefix(bs.Text(), "# END deps") {
			sawEnd = true
		}
		if sawEnd {
			footBuf.Write(bs.Bytes())
			footBuf.WriteByte('\n')
		}
	}
	if err := bs.Err(); err != nil {
		log.Fatalf("error parsing %s: %v", *updateFile, err)
	}
	if !sawBegin {
		log.Fatalf(`file %s is missing a "# BEGIN deps" line`, *updateFile)
	}
	if !sawEnd {
		log.Fatalf(`file %s is missing a "# END deps" line`, *updateFile)
	}
	return headBuf.Bytes(), footBuf.Bytes()
}

func main() {
	flag.Parse()
	ignorePrefixes = strings.Split(*ignorePrefixFlag, ",")
	if flag.NArg() != 1 {
		log.SetFlags(0)
		log.Fatalf("Usage: gitlock <package>")
	}
	mainPkg := flag.Arg(0) // not necessary "package main", but almost certainly.
	header, footer := parseDockerFile()

	depOut, err := exec.Command("go", "list",
		"-tags="+*tags,
		"-f", "{{range .Deps}}{{.}}\n{{end}}", mainPkg).Output()
	if err != nil {
		log.Fatalf("listing deps of %q: %v", mainPkg, formatExecErr(err))
	}

	var deps []string
	for _, pkg := range strings.Split(string(depOut), "\n") {
		if ignorePkg(pkg) {
			continue
		}
		deps = append(deps, pkg)
	}
	sort.Strings(deps)

	// Build a map of root git dir => packages using that root git dir.
	var gitDirPkgs = map[string][]string{}
	for _, pkg := range deps {
		buildPkg, err := build.Import(pkg, "", build.FindOnly)
		if err != nil {
			log.Fatalf("importing %s: %v", pkg, err)
		}
		pkgDir := buildPkg.Dir
		gitDir, err := findGitDir(pkgDir)
		if err != nil {
			log.Fatalf("finding git dir of %s: %v", pkgDir, err)
		}
		gitDirPkgs[gitDir] = append(gitDirPkgs[gitDir], pkg)
	}

	// Sorted list of unique git root dirs.
	var gitDirs []string
	for d := range gitDirPkgs {
		gitDirs = append(gitDirs, d)
	}
	sort.Strings(gitDirs)

	var buf bytes.Buffer
	var out io.Writer = os.Stdout
	if *updateFile != "" {
		buf.Write(header)
		buf.WriteByte('\n')
		out = &buf
	}
	for _, gitDir := range gitDirs {
		cmd := exec.Command("git", "log", "-n", "1", "--pretty=%H %ci")
		cmd.Dir = gitDir
		stdout, err := cmd.Output()
		if err != nil {
			log.Fatal(err)
		}
		f := strings.SplitN(strings.TrimSpace(string(stdout)), " ", 2)
		hash, ymd := f[0], f[1][:10]

		repoName := gitDir[strings.Index(gitDir, "/src/")+5:]

		var comment string
		if n := len(gitDirPkgs[gitDir]); n > 1 {
			comment = fmt.Sprintf(" `#and %d other pkgs`", n)
		}
		fmt.Fprintf(out, "# Repo %s at %s (%s)\n", repoName, hash[:7], ymd)
		fmt.Fprintf(out, "ENV REV=%s\n", hash)
		fmt.Fprintf(out, "RUN go get -d %s%s &&\\\n", gitDirPkgs[gitDir][0], comment)
		fmt.Fprintf(out, "    (cd /go/src/%s && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)\n\n", repoName)
	}

	fmt.Fprintf(out, "# Optimization to speed up iterative development, not necessary for correctness:\n")
	fmt.Fprintf(out, "RUN go install %s\n", strings.Join(deps, " \\\n\t"))

	if *updateFile == "" {
		return
	}
	buf.Write(footer)
	if err := ioutil.WriteFile(*updateFile, buf.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}
}

func findGitDir(dir string) (string, error) {
	dir0 := dir
	var lastDir string
	for dir != "" && dir != "." && dir != lastDir {
		fi, err := os.Stat(filepath.Join(dir, ".git"))
		if err == nil && fi.IsDir() {
			return dir, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		lastDir = dir
		dir = filepath.Dir(dir)
	}
	return "", fmt.Errorf("no git dir found for %s", dir0)
}

// ignorePkg reports whether pkg, if it appears as a dependency of the
// root package, should be omitted from the output.
func ignorePkg(pkg string) bool {
	if strings.HasPrefix(pkg, "vendor/") || !strings.Contains(pkg, ".") {
		return true
	}
	for _, pfx := range ignorePrefixes {
		if strings.HasPrefix(pkg, pfx) {
			return true
		}
	}
	return false
}

func formatExecErr(err error) string {
	if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
		return fmt.Sprintf("%s: %s", err, ee.Stderr)
	}
	return fmt.Sprint(err)
}
