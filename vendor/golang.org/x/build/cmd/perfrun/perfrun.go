// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// perfrun interacts with the buildlet coordinator to run the go1
// benchmarks on a buildlet slave for the most recent successful
// commits according to the build dashboard.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/build/buildenv"
	"golang.org/x/build/buildlet"
	"golang.org/x/build/types"
)

var (
	buildletBench = flag.String("buildlet", "", "name of buildlet to use for benchmarks")
	buildletSrc   = flag.String("buildlet_src", "", "name of builder to get binaries from (defaults to the same as buildlet)")
	buildEnv      *buildenv.Environment
)

// runBench runs the benchmarks from each of the revisions in
// commits. It uses the tarballs built by the "src" buildlet and runs
// the benchmarks on the "bench" buildlet. It writes a log to out in
// the standard benchmark format
// (https://github.com/golang/proposal/blob/master/design/14313-benchmark-format.md).
func runBench(out io.Writer, bench, src string, commits []string) error {
	bc, err := namedClient(*buildletBench)
	if err != nil {
		return err
	}
	log.Printf("Using buildlet %s", bc.RemoteName())
	workDir, err := bc.WorkDir()
	if err != nil {
		log.Printf("Getting WorkDir: %v", err)
		return err
	}
	for _, rev := range commits {
		log.Printf("Installing prebuilt rev %s", rev)
		dir := fmt.Sprintf("go-%s", rev)
		// Copy pre-built trees
		if err := bc.PutTarFromURL(buildEnv.SnapshotURL(src, rev), dir); err != nil {
			log.Printf("failed to extract snapshot for %s: %v", rev, err)
			return err
		}
		// Build binaries
		log.Printf("Building bench binary for rev %s", rev)
		var buf bytes.Buffer
		remoteErr, err := bc.Exec(path.Join(dir, "bin", "go"), buildlet.ExecOpts{
			Output:   &buf,
			ExtraEnv: []string{"GOROOT=" + path.Join(workDir, dir)},
			Args:     []string{"test", "-c"},
			Dir:      path.Join(dir, "test/bench/go1"),
		})
		if remoteErr != nil {
			log.Printf("failed to compile bench for %s: %v", rev, remoteErr)
			log.Printf("output: %s", buf.Bytes())
			return remoteErr
		}
		if err != nil {
			log.Printf("Exec error: %v", err)
			log.Printf("output: %s", buf.Bytes())
			return err
		}
	}
	// Loop over commits and run N times interleaved, grabbing output
	// TODO: Overhead of multiple Exec calls might be significant; should we ship over a shell script to do this in one go?
	for i := 0; i < 10; i++ {
		log.Printf("Starting bench run %d", i)
		for _, rev := range commits {
			var buf bytes.Buffer
			remoteErr, err := bc.Exec(path.Join("go-"+rev, "test/bench/go1/go1.test"), buildlet.ExecOpts{
				Output: &buf,
				Args:   []string{"-test.bench", ".", "-test.benchmem"},
			})
			if remoteErr != nil {
				log.Printf("failed to run %d-%s: %v", i, rev, remoteErr)
				log.Printf("output: %s", buf.Bytes())
				return remoteErr
			}
			if err != nil {
				log.Printf("Exec error: %v", err)
				log.Printf("output: %s", buf.Bytes())
				return err
			}
			log.Printf("%d-%s: %s", i, rev, buf.Bytes()) // XXX
			fmt.Fprintf(out, "commit: %s\niteration: %d\nstart: %s", rev, i, time.Now().UTC().Format(time.RFC3339))
			out.Write(buf.Bytes())
			out.Write([]byte{'\n'})
		}
	}

	// Destroy client
	// TODO: defer this so we don't leak clients?
	if err := bc.Close(); err != nil {
		return err
	}
	return nil
}

func namedClient(name string) (*buildlet.Client, error) {
	if strings.Contains(name, ":") {
		return buildlet.NewClient(name, buildlet.NoKeyPair), nil
	}
	cc, err := buildlet.NewCoordinatorClientFromFlags()
	if err != nil {
		return nil, err
	}
	return cc.CreateBuildlet(name)
	// TODO(quentin): Figure out a way to detect if there's an already running builder with this name.
	//return cc.NamedBuildlet(name)
}

// findCommits finds all the recent successful commits for the given builder
func findCommits(name string) ([]string, error) {
	var bs types.BuildStatus
	res, err := http.Get(buildEnv.DashBase() + "?mode=json")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&bs); err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected http status %v", res.Status)
	}

	var commits []string

	for builderIdx := 0; builderIdx < len(bs.Builders); builderIdx++ {
		if bs.Builders[builderIdx] == name {
			for _, br := range bs.Revisions {
				if br.Repo != "go" {
					// Only process go repo for now
					continue
				}
				if br.Results[builderIdx] == "ok" {
					commits = append(commits, br.Revision)
				}
			}
			return commits, nil
		}
	}
	return nil, fmt.Errorf("builder %q not found", name)
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage of perfrun: perfrun [flags] <commits>

Flags:
`)
	flag.PrintDefaults()
	os.Exit(1)
}

func main() {
	buildlet.RegisterFlags()
	flag.Usage = usage
	flag.Parse()
	buildEnv = buildenv.FromFlags()
	args := flag.Args()
	if *buildletBench == "" {
		usage()
	}
	if *buildletSrc == "" {
		*buildletSrc = *buildletBench
	}
	if len(args) == 0 {
		res, err := findCommits(*buildletSrc)
		args = res
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed finding commits to build: %v", err)
			os.Exit(1)
		}
	}
	log.Printf("Running bench on %v", args)
	out, err := os.Create(fmt.Sprintf("perfrun-%s-%s-%s.log", *buildletSrc, *buildletBench, time.Now().Format("20060102150405")))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Creating log failed: %v", err)
	}
	defer func() {
		if err := out.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed writing log: %v", err)
			os.Exit(1)
		}
	}()
	if err := runBench(out, *buildletBench, *buildletSrc, args); err != nil {
		fmt.Fprintf(os.Stderr, "Failed running bench: %v", err)
		os.Exit(1)
	}
}
