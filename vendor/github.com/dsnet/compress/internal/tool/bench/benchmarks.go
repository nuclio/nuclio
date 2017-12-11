// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dsnet/compress/internal/testutil"
)

type Result struct {
	R float64 // Rate (MB/s) or ratio (rawSize/compSize)
	D float64 // Delta ratio relative to primary benchmark
}

// BenchmarkEncoder benchmarks a single encoder on the given input data using
// the selected compression level and reports the result.
func BenchmarkEncoder(input []byte, enc Encoder, lvl int) testing.BenchmarkResult {
	return testing.Benchmark(func(b *testing.B) {
		b.StopTimer()
		if enc == nil {
			b.Fatalf("unexpected error: nil Encoder")
		}
		runtime.GC()
		b.StartTimer()
		for i := 0; i < b.N; i++ {
			wr := enc(ioutil.Discard, lvl)
			_, err := io.Copy(wr, bytes.NewBuffer(input))
			if err := wr.Close(); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
			b.SetBytes(int64(len(input)))
		}
	})
}

// BenchmarkEncoderSuite runs multiple benchmarks across all encoder
// implementations, files, levels, and sizes.
//
// The values returned have the following structure:
//	results: [len(files)*len(levels)*len(sizes)][len(encs)]Result
//	names:   [len(files)*len(levels)*len(sizes)]string
func BenchmarkEncoderSuite(ft Format, encs []string, files []file, levels, sizes []int, tick func()) (results [][]Result, names []string) {
	return benchmarkSuite(encs, files, levels, sizes, tick,
		func(input []byte, enc string, lvl int) Result {
			result := BenchmarkEncoder(input, encoders[ft][enc], lvl)
			if result.N == 0 {
				return Result{}
			}
			us := (float64(result.T.Nanoseconds()) / 1e3) / float64(result.N)
			rate := float64(result.Bytes) / us
			return Result{R: rate}
		})
}

// BenchmarkDecoder benchmarks a single decoder on the given pre-compressed
// input data and reports the result.
func BenchmarkDecoder(input []byte, dec Decoder) testing.BenchmarkResult {
	return testing.Benchmark(func(b *testing.B) {
		b.StopTimer()
		if dec == nil {
			b.Fatalf("unexpected error: nil Decoder")
		}
		runtime.GC()
		b.StartTimer()
		for i := 0; i < b.N; i++ {
			rd := dec(bufio.NewReader(bytes.NewBuffer(input)))
			cnt, err := io.Copy(ioutil.Discard, rd)
			if err := rd.Close(); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
			b.SetBytes(cnt)
		}
	})
}

// BenchmarkDecoderSuite runs multiple benchmarks across all decoder
// implementations, files, levels, and sizes.
//
// The values returned have the following structure:
//	results: [len(files)*len(levels)*len(sizes)][len(decs)]Result
//	names:   [len(files)*len(levels)*len(sizes)]string
func BenchmarkDecoderSuite(ft Format, decs []string, files []file, levels, sizes []int, ref Encoder, tick func()) (results [][]Result, names []string) {
	return benchmarkSuite(decs, files, levels, sizes, tick,
		func(input []byte, dec string, lvl int) Result {
			buf := new(bytes.Buffer)
			wr := ref(buf, lvl)
			if _, err := io.Copy(wr, bytes.NewReader(input)); err != nil {
				return Result{}
			}
			if wr.Close() != nil {
				return Result{}
			}
			output := buf.Bytes()

			result := BenchmarkDecoder(output, decoders[ft][dec])
			if result.N == 0 {
				return Result{}
			}
			us := (float64(result.T.Nanoseconds()) / 1e3) / float64(result.N)
			rate := float64(result.Bytes) / us
			return Result{R: rate}
		})
}

// BenchmarkRatioSuite runs multiple benchmarks across all encoder
// implementations, files, levels, and sizes.
//
// The values returned have the following structure:
//	results: [len(files)*len(levels)*len(sizes)][len(encs)]Result
//	names:   [len(files)*len(levels)*len(sizes)]string
func BenchmarkRatioSuite(ft Format, encs []string, files []file, levels, sizes []int, tick func()) (results [][]Result, names []string) {
	return benchmarkSuite(encs, files, levels, sizes, tick,
		func(input []byte, enc string, lvl int) Result {
			buf := new(bytes.Buffer)
			wr := encoders[ft][enc](buf, lvl)
			if _, err := io.Copy(wr, bytes.NewReader(input)); err != nil {
				return Result{}
			}
			if wr.Close() != nil {
				return Result{}
			}
			output := buf.Bytes()
			ratio := float64(len(input)) / float64(len(output))
			return Result{R: ratio}
		})
}

type benchFunc func(input []byte, codec string, level int) Result

func benchmarkSuite(codecs []string, files []file, levels, sizes []int, tick func(), run benchFunc) ([][]Result, []string) {
	// Allocate buffers for the result.
	d0 := len(files) * len(levels) * len(sizes)
	d1 := len(codecs)
	results := make([][]Result, d0)
	for i := range results {
		results[i] = make([]Result, d1)
	}
	names := make([]string, d0)

	// Run the benchmark for every codec, file, level, and size.
	var i int
	for _, f := range files {
		for _, l := range levels {
			for _, n := range sizes {
				b, err := ioutil.ReadFile(f.Abs)
				if err == nil {
					b = testutil.ResizeData(b, n)
				}
				fname := strings.Replace(f.Rel, string(filepath.Separator), "_", -1)
				name := fmt.Sprintf("%s:%d:%s", fname, l, intName(len(b)))
				for j, c := range codecs {
					if tick != nil {
						tick()
					}
					names[i] = name
					if err == nil {
						results[i][j] = run(b, c, l)
					}
					results[i][j].D = results[i][j].R / results[i][0].R
				}
				i++
			}
		}
	}
	return results, names
}
