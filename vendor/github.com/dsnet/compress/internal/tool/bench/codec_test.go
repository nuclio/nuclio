// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/dsnet/compress/internal/testutil"
)

// The unit tests can also be used to quickly test all of the implementations
// with respect to each other for correctness. The command-line flags can be
// used to specify any arbitrary corpus of test data to use.
//
// Example usage:
//	$ go test -c
//	$ ./bench.test \
//		-paths    $CORPUS_PATH   \
//		-globs    "*.txt:*.bin"  \
//		-test.run "//fl/std|cgo" \
//		-test.v

var level int

func TestMain(m *testing.M) {
	setDefaults()
	flag.Var(&paths, "paths", "List of paths to search for test files")
	flag.Var(&globs, "globs", "List of globs to match for test files")
	flag.IntVar(&level, "level", 6, "Default compression level to use")
	flag.Parse()
	os.Exit(m.Run())
}

type semaphore chan struct{}

func newSemaphore(n int) semaphore { return make(chan struct{}, n) }
func (s *semaphore) Acquire()      { *s <- struct{}{} }
func (s *semaphore) Release()      { <-*s }

// Each sub-test is run in a goroutine so that we can have fine control over
// exactly how many sub-tests are running. When running over a large corpus,
// this helps prevent all the sub-tests from executing at once and OOMing
// the machine. The semaphores below control the maximum number of concurrent
// operations that can be running for each dimension.
//
// We avoid using t.Parallel since that causes t.Run to return immediately and
// does not provide the caller with feedback that all sub-operations completed.
// This causes the next operation to prematurely start, leading to overloads.
var (
	semaFiles    = newSemaphore(runtime.NumCPU())
	semaFormats  = newSemaphore(runtime.NumCPU())
	semaEncoders = newSemaphore(runtime.NumCPU())
	semaDecoders = newSemaphore(runtime.NumCPU())
)

// TestCodecs tests that the output of each registered encoder is a valid input
// for each registered decoder. This test runs in O(n^2) where n is the number
// of registered codecs. This assumes that the number of test files and
// compression formats stays relatively constant.
func TestCodecs(t *testing.T) {
	var wg sync.WaitGroup
	defer wg.Wait()
	for _, fi := range getFiles(paths, globs) {
		fi := fi
		name := "File:" + strings.Replace(fi.Rel, string(filepath.Separator), "_", -1)
		goRun(t, &wg, &semaFiles, name, func(t *testing.T) {
			dd := testutil.MustLoadFile(fi.Abs)
			testFormats(t, dd)
		})
	}
}

func testFormats(t *testing.T, dd []byte) {
	var wg sync.WaitGroup
	defer wg.Wait()
	for _, ft := range formats {
		ft := ft
		name := "Format:" + enumToFmt[ft]
		goRun(t, &wg, &semaFormats, name, func(t *testing.T) {
			if len(encoders[ft]) == 0 || len(decoders[ft]) == 0 {
				t.Skip("no codecs available")
			}
			testEncoders(t, ft, dd)
		})
	}
}

func testEncoders(t *testing.T, ft Format, dd []byte) {
	var wg sync.WaitGroup
	defer wg.Wait()
	for encName := range encoders[ft] {
		encName := encName
		name := "Encoder:" + encName
		goRun(t, &wg, &semaEncoders, name, func(t *testing.T) {
			be := new(bytes.Buffer)
			zw := encoders[ft][encName](be, level)
			if _, err := io.Copy(zw, bytes.NewReader(dd)); err != nil {
				t.Fatalf("unexpected Write error: %v", err)
			}
			if err := zw.Close(); err != nil {
				t.Fatalf("unexpected Close error: %v", err)
			}
			de := be.Bytes()
			testDecoders(t, ft, dd, de)
		})
	}
}

func testDecoders(t *testing.T, ft Format, dd, de []byte) {
	var wg sync.WaitGroup
	defer wg.Wait()
	for decName := range decoders[ft] {
		decName := decName
		name := "Decoder:" + decName
		goRun(t, &wg, &semaDecoders, name, func(t *testing.T) {
			bd := new(bytes.Buffer)
			zr := decoders[ft][decName](bytes.NewReader(de))
			if _, err := io.Copy(bd, zr); err != nil {
				t.Fatalf("unexpected Read error: %v", err)
			}
			if err := zr.Close(); err != nil {
				t.Fatalf("unexpected Close error: %v", err)
			}
			if got, want, ok := testutil.BytesCompare(bd.Bytes(), dd); !ok {
				t.Errorf("data mismatch:\ngot  %s\nwant %s", got, want)
			}
		})
	}
}

func goRun(t *testing.T, wg *sync.WaitGroup, sm *semaphore, name string, fn func(t *testing.T)) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		t.Run(name, func(t *testing.T) {
			sm.Acquire()
			defer sm.Release()
			defer recoverPanic(t)
			fn(t)
		})
	}()
}

func recoverPanic(t *testing.T) {
	if ex := recover(); ex != nil {
		t.Fatalf("unexpected panic: %v", ex)
	}
}
