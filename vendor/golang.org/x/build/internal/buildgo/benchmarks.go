// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package buildgo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"golang.org/x/build/buildenv"
	"golang.org/x/build/buildlet"
	"golang.org/x/build/cmd/coordinator/spanlog"
	"golang.org/x/build/dashboard"
	"golang.org/x/build/internal/sourcecache"
)

// benchRuns is the number of times to run each benchmark binary
const benchRuns = 5

// BenchmarkItem represents a single package's benchmarks, to be run on one or more commit.
// After Run is called, the output of each commit can be retrieved from the Output field.
type BenchmarkItem struct {
	binary   string   // name of binary relative to goroot
	args     []string // args to run binary with
	preamble string   // string to print before benchmark results (e.g. "pkg: test/bench/go1\n")
	dir      string   // optional directory (relative to $WORKDIR) to run benchmarks in
	Output   []string // benchmark output for each commit

	build func(bc *buildlet.Client, goroot string, w io.Writer) (remoteErr, err error) // how to build benchmark binary
}

// Name returns a string that uniquely identifies this benchmark.
func (b *BenchmarkItem) Name() string {
	return b.binary + " " + strings.Join(b.args, " ")
}

// buildGo1 builds the Go 1 benchmarks.
func buildGo1(conf *dashboard.BuildConfig, bc *buildlet.Client, goroot string, w io.Writer) (remoteErr, err error) {
	workDir, err := bc.WorkDir()
	if err != nil {
		return nil, err
	}
	var found bool
	if err := bc.ListDir(path.Join(goroot, "test/bench/go1"), buildlet.ListDirOpts{}, func(e buildlet.DirEntry) {
		switch e.Name() {
		case "go1.test", "go1.test.exe":
			found = true
		}
	}); err != nil {
		return nil, err
	}
	if found {
		return nil, nil
	}
	return bc.Exec(path.Join(goroot, "bin", "go"), buildlet.ExecOpts{
		Output:   w,
		ExtraEnv: []string{"GOROOT=" + conf.FilePathJoin(workDir, goroot)},
		Args:     []string{"test", "-c"},
		Dir:      path.Join(goroot, "test/bench/go1"),
	})
}

// buildPkg builds a package's benchmarks.
func buildPkg(conf *dashboard.BuildConfig, bc *buildlet.Client, goroot string, w io.Writer, pkg, name string) (remoteErr, err error) {
	workDir, err := bc.WorkDir()
	if err != nil {
		return nil, err
	}
	return bc.Exec(path.Join(goroot, "bin", "go"), buildlet.ExecOpts{
		Output:   w,
		ExtraEnv: []string{"GOROOT=" + conf.FilePathJoin(workDir, goroot)},
		Args:     []string{"test", "-c", "-o", conf.FilePathJoin(workDir, goroot, name), pkg},
	})
}

// buildXBenchmark builds a benchmark from x/benchmarks.
func buildXBenchmark(sl spanlog.Logger, conf *dashboard.BuildConfig, bc *buildlet.Client, goroot string, w io.Writer, rev, pkg, name string) (remoteErr, err error) {
	workDir, err := bc.WorkDir()
	if err != nil {
		return nil, err
	}
	if err := bc.ListDir("gopath/src/golang.org/x/benchmarks", buildlet.ListDirOpts{}, func(buildlet.DirEntry) {}); err != nil {
		if err := FetchSubrepo(sl, bc, "benchmarks", rev); err != nil {
			return nil, err
		}
	}
	return bc.Exec(path.Join(goroot, "bin/go"), buildlet.ExecOpts{
		Output: w,
		ExtraEnv: []string{
			"GOROOT=" + conf.FilePathJoin(workDir, goroot),
			"GOPATH=" + conf.FilePathJoin(workDir, "gopath"),
		},
		Args: []string{"build", "-o", conf.FilePathJoin(workDir, goroot, name), pkg},
	})
}

// EnumerateBenchmarks returns a slice of the benchmarks to be run for the built Go distribution found in gb.Goroot.
// If benchmarksRev is non-empty, it is the revision of x/benchmarks to check out for additional benchmarks.
// pkgs contains a list of possibly duplicate packages that will be searched for benchmarks.
func (gb GoBuilder) EnumerateBenchmarks(bc *buildlet.Client, benchmarksRev string, pkgs []string) ([]*BenchmarkItem, error) {
	workDir, err := bc.WorkDir()
	if err != nil {
		err = fmt.Errorf("buildBench, WorkDir: %v", err)
		return nil, err
	}

	// Fetch x/benchmarks
	if benchmarksRev != "" {
		if err := FetchSubrepo(gb.Logger, bc, "benchmarks", benchmarksRev); err != nil {
			return nil, err
		}
	}

	var out []*BenchmarkItem

	// These regexes shard the go1 tests so each shard takes about 20s, ensuring no test runs for
	for _, re := range []string{`^Benchmark[BF]`, `^Benchmark[HR]`, `^Benchmark[^BFHR]`} {
		out = append(out, &BenchmarkItem{
			binary:   "test/bench/go1/go1.test",
			args:     []string{"-test.bench", re, "-test.benchmem"},
			preamble: "pkg: test/bench/go1\n",
			build: func(bc *buildlet.Client, goroot string, w io.Writer) (error, error) {
				return buildGo1(gb.Conf, bc, goroot, w)
			},
		})
	}

	// Enumerate x/benchmarks
	if benchmarksRev != "" {
		var buf bytes.Buffer
		remoteErr, err := bc.Exec(path.Join(gb.Goroot, "bin/go"), buildlet.ExecOpts{
			Output: &buf,
			ExtraEnv: []string{
				"GOROOT=" + gb.Conf.FilePathJoin(workDir, gb.Goroot),
				"GOPATH=" + gb.Conf.FilePathJoin(workDir, "gopath"),
			},
			Args: []string{"list", "-f", `{{if eq .Name "main"}}{{.ImportPath}}{{end}}`, "golang.org/x/benchmarks/..."},
		})
		if remoteErr != nil {
			return nil, remoteErr
		}
		if err != nil {
			return nil, err
		}
		for _, pkg := range strings.Fields(buf.String()) {
			pkg := pkg
			name := "bench-" + path.Base(pkg) + ".exe"
			out = append(out, &BenchmarkItem{
				binary: name, args: nil, build: func(bc *buildlet.Client, goroot string, w io.Writer) (error, error) {
					return buildXBenchmark(gb.Logger, gb.Conf, bc, goroot, w, benchmarksRev, pkg, name)
				}})
		}
	}
	// Enumerate package benchmarks that were affected by the CL
	if len(pkgs) > 0 {
		// Find packages that actually have benchmarks or tests.
		var buf bytes.Buffer
		remoteErr, err := bc.Exec(path.Join(gb.Goroot, "bin/go"), buildlet.ExecOpts{
			Output: &buf,
			ExtraEnv: []string{
				"GOROOT=" + gb.Conf.FilePathJoin(workDir, gb.Goroot),
			},
			Args: append([]string{"list", "-e", "-f", "{{if or (len .TestGoFiles) (len .XTestGoFiles)}}{{.ImportPath}}{{end}}"}, pkgs...),
		})
		if remoteErr != nil {
			return nil, remoteErr
		}
		if err != nil {
			return nil, err
		}

		for _, pkg := range strings.Fields(buf.String()) {
			// Some packages have large numbers of benchmarks.
			// To avoid running benchmarks for hours and hours, we exclude runtime (which has 350+ benchmarks) and run benchmarks for .1s instead of the default 1s.
			// This allows the remaining standard library packages to run a single iteration of a package's benchmarks in <20s, making them have the same scale as go1 benchmark shards.
			if pkg == "runtime" {
				continue
			}
			name := "bench-" + strings.Replace(pkg, "/", "-", -1) + ".exe"
			out = append(out, &BenchmarkItem{
				binary: name,
				dir:    path.Join(gb.Goroot, "src", pkg),
				args:   []string{"-test.bench", ".", "-test.benchmem", "-test.run", "^$", "-test.benchtime", "100ms"},
				build: func(bc *buildlet.Client, goroot string, w io.Writer) (error, error) {
					return buildPkg(gb.Conf, bc, goroot, w, pkg, name)
				}})
		}
	}
	return out, nil
}

// runOneBenchBinary runs a binary on the buildlet and writes its output to w with a trailing newline.
//
// TODO: this signature is too big. Make it a method of something?
func runOneBenchBinary(conf *dashboard.BuildConfig, bc *buildlet.Client, w io.Writer, goroot, dir, binaryPath string, args []string) (remoteErr, err error) {
	defer w.Write([]byte{'\n'})
	workDir, err := bc.WorkDir()
	if err != nil {
		return nil, fmt.Errorf("runOneBenchBinary, WorkDir: %v", err)
	}
	// Some benchmarks need GOROOT so they can invoke cmd/go.
	return bc.Exec(binaryPath, buildlet.ExecOpts{
		Output: w,
		Dir:    dir,
		Args:   args,
		Path:   []string{"$WORKDIR/" + goroot + "/bin", "$PATH"},
		ExtraEnv: []string{
			"GOROOT=" + conf.FilePathJoin(workDir, goroot),
			// Some builders run in virtualization
			// environments (GCE, GKE, etc.). These
			// environments have CPU antagonists - by
			// limiting GOMAXPROCS to 2 we can reduce the
			// variability of benchmarks by leaving free
			// cores available for antagonists. We don't
			// want GOMAXPROCS=1 because that invokes
			// special runtime behavior. Test data is at
			// https://perf.golang.org/search?q=upload%3A20170512.4+cores%3Aall+parallel%3A4+%7C+gomaxprocs%3A2+vs+gomaxprocs%3A16+vs+gomaxprocs%3A32
			"GOMAXPROCS=2",
		},
	})
}

func buildRev(buildEnv *buildenv.Environment, sl spanlog.Logger, conf *dashboard.BuildConfig, bc *buildlet.Client, w io.Writer, goroot string, br BuilderRev) error {
	if br.SnapshotExists(context.TODO(), buildEnv) {
		return bc.PutTarFromURL(br.SnapshotURL(buildEnv), goroot)
	}
	if err := bc.PutTar(VersionTgz(br.Rev), goroot); err != nil {
		return err
	}
	srcTar, err := sourcecache.GetSourceTgz(sl, "go", br.Rev)
	if err != nil {
		return err
	}
	if err := bc.PutTar(srcTar, goroot); err != nil {
		return err
	}
	builder := GoBuilder{
		Logger:     sl,
		BuilderRev: br,
		Conf:       conf,
		Goroot:     goroot,
	}
	remoteErr, err := builder.RunMake(bc, w)
	if err != nil {
		return err
	}
	return remoteErr
}

// Run runs all the iterations of this benchmark on bc.
// Build output is sent to w. Benchmark output is stored in b.output.
// revs must contain exactly two revs. The first rev is assumed to be present in "go", and the second will be placed into "go-parent".
// TODO(quentin): Support len(revs) != 2.
func (b *BenchmarkItem) Run(buildEnv *buildenv.Environment, sl spanlog.Logger, conf *dashboard.BuildConfig, bc *buildlet.Client, w io.Writer, revs []BuilderRev) (remoteErr, err error) {
	// Ensure we have a built parent repo.
	if err := bc.ListDir("go-parent", buildlet.ListDirOpts{}, func(buildlet.DirEntry) {}); err != nil {
		pbr := revs[1]
		sp := sl.CreateSpan("bench_build_parent", bc.Name())
		err = buildRev(buildEnv, sl, conf, bc, w, "go-parent", pbr)
		sp.Done(err)
		if err != nil {
			return nil, err
		}
	}
	// Build benchmark.
	for _, goroot := range []string{"go", "go-parent"} {
		sp := sl.CreateSpan("bench_build", fmt.Sprintf("%s/%s: %s", goroot, b.binary, bc.Name()))
		remoteErr, err = b.build(bc, goroot, w)
		sp.Done(err)
		if remoteErr != nil || err != nil {
			return remoteErr, err
		}
	}

	type commit struct {
		path string
		out  bytes.Buffer
	}
	commits := []*commit{
		{path: "go-parent"},
		{path: "go"},
	}

	for _, c := range commits {
		c.out.WriteString(b.preamble)
	}

	// Run bench binaries and capture the results
	for i := 0; i < benchRuns; i++ {
		for _, c := range commits {
			fmt.Fprintf(&c.out, "iteration: %d\nstart-time: %s\n", i, time.Now().UTC().Format(time.RFC3339))
			binaryPath := path.Join(c.path, b.binary)
			sp := sl.CreateSpan("run_one_bench", binaryPath)
			remoteErr, err = runOneBenchBinary(conf, bc, &c.out, c.path, b.dir, binaryPath, b.args)
			sp.Done(err)
			if err != nil || remoteErr != nil {
				c.out.WriteTo(w)
				if err != nil {
					fmt.Fprintf(w, "execution error: %v\n", err)
				} else if remoteErr != nil {
					fmt.Fprintf(w, "remote error: %v\n", remoteErr)
				}
				return
			}
		}
	}
	b.Output = []string{
		commits[0].out.String(),
		commits[1].out.String(),
	}
	return nil, nil
}
