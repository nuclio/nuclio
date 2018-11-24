// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The gostats command computes stats about the Go project.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"runtime"
	"sort"
	"strings"
	"time"

	"golang.org/x/build/maintner"
	"golang.org/x/build/maintner/godata"
)

var (
	loadStats = flag.Bool("time-load", false, "time the load of the corpus")
	topFiles  = flag.Int("modified-files", 0, "if non-zero, show the top modified files")
)

type FileCount struct {
	File  string
	Count int
}

func topModifiedFiles(gp *maintner.GerritProject, topN int) []FileCount {
	n := map[string]int{} // file -> count
	gp.ForeachCLUnsorted(func(gcl *maintner.GerritCL) error {
		for _, f := range gcl.Commit.Files {
			n[modernizeFilename(f.File)]++
		}
		return nil
	})
	files := make([]FileCount, 0, len(n))
	for file, count := range n {
		files = append(files, FileCount{file, count})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Count > files[j].Count
	})
	if len(files) > topN {
		files = files[:topN]
	}
	return files
}

func modernizeFilename(f string) string {
	if strings.HasPrefix(f, "src/pkg/") {
		f = "src/" + strings.TrimPrefix(f, "src/pkg/")
	}
	if strings.HasPrefix(f, "src/http/") {
		f = "src/net/http/" + strings.TrimPrefix(f, "src/http/")
	}
	return f
}

var errStop = errors.New("stop")

func main() {
	flag.Parse()

	t0 := time.Now()
	corpus, err := godata.Get(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	if *loadStats {
		dur := time.Since(t0)
		runtime.GC()
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		log.Printf("Loaded data in %v. Memory: %v MB", dur, ms.HeapAlloc>>20)
	}

	if *topFiles > 0 {
		gerrit := corpus.Gerrit()
		proj := gerrit.Project("go.googlesource.com", "go")
		if proj == nil {
			panic("godata.Get did not fetch go.googlesource.com/go")
		}
		top := topModifiedFiles(proj, *topFiles)
		for _, fc := range top {
			fmt.Printf(" %5d %s\n", fc.Count, fc.File)
		}
		return
	}
}
