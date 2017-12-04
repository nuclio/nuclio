// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Benchmark tool to compare performance between multiple compression
// implementations. Individual implementations are referred to as codecs.
//
// Example usage:
//	$ go build
//	$ ./bench \
//		-formats fl              \
//		-tests   encRate,decRate \
//		-codecs  std,ds,cgo      \
//		-globs   twain.txt       \
//		-levels  1,6,9           \
//		-sizes   1e4,1e5,1e6
//
//
//	BENCHMARK: fl:encRate
//		benchmark             std MB/s  delta      cgo MB/s  delta
//		twain.txt:1:1e4           9.88  1.00x         48.89  4.95x
//		twain.txt:1:1e5          26.70  1.00x         64.99  2.43x
//		twain.txt:1:1e6          31.95  1.00x         65.56  2.05x
//		twain.txt:6:1e4           7.31  1.00x         30.67  4.19x
//		twain.txt:6:1e5           8.33  1.00x         17.22  2.07x
//		twain.txt:6:1e6           8.05  1.00x         15.99  1.99x
//		twain.txt:9:1e4           8.15  1.00x         30.04  3.69x
//		twain.txt:9:1e5           6.59  1.00x         12.82  1.95x
//		twain.txt:9:1e6           6.32  1.00x         11.40  1.80x
//
//	BENCHMARK: fl:decRate
//		benchmark             std MB/s  delta      ds MB/s  delta      cgo MB/s  delta
//		twain.txt:1:1e4          49.61  1.00x        74.15  1.49x        163.81  3.30x
//		twain.txt:1:1e5          60.25  1.00x        91.25  1.51x        177.38  2.94x
//		twain.txt:1:1e6          61.75  1.00x        95.82  1.55x        181.11  2.93x
//		twain.txt:6:1e4          52.16  1.00x        77.25  1.48x        174.30  3.34x
//		twain.txt:6:1e5          72.23  1.00x       108.01  1.50x        195.31  2.70x
//		twain.txt:6:1e6          76.59  1.00x       116.80  1.53x        203.88  2.66x
//		twain.txt:9:1e4          52.97  1.00x        77.58  1.46x        172.88  3.26x
//		twain.txt:9:1e5          72.35  1.00x       108.37  1.50x        197.15  2.72x
//		twain.txt:9:1e6          76.82  1.00x       118.02  1.54x        204.87  2.67x
//
//
//	RUNTIME: 2m42.434570856s
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	flag.CommandLine.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.CommandLine.VisitAll(func(f *flag.Flag) {
			if strings.HasPrefix(f.Name, "test.") {
				return
			}
			typ, usage := flag.UnquoteUsage(f)
			s := fmt.Sprintf("    -%s %s\n        %s (default %q)\n",
				f.Name, typ, usage, f.DefValue)
			fmt.Fprint(os.Stderr, s)
		})
	}

	setDefaults()
	flag.Var(&formats, "formats", "List of formats to benchmark")
	flag.Var(&tests, "tests", "List of different benchmark tests")
	flag.Var(&codecs, "codecs", "List of codecs to benchmark")
	flag.Var(&paths, "paths", "List of paths to search for test files")
	flag.Var(&globs, "globs", "List of globs to match for test files")
	flag.Var(&levels, "levels", "List of compression levels to benchmark")
	flag.Var(&sizes, "sizes", "List of input sizes to benchmark")
	flag.Parse()

	files := getFiles(paths, globs)

	ts := time.Now()
	runBenchmarks(files, codecs, formats, tests, levels, sizes)
	te := time.Now()
	fmt.Printf("RUNTIME: %v\n", te.Sub(ts))
}

type file struct{ Abs, Rel string }

// getFiles returns a list of files found by applying the glob matching on
// all the specified paths. This function ignores any errors.
func getFiles(paths []string, globs []string) []file {
	var fs []file
	for _, p := range paths {
		for _, g := range globs {
			ms, _ := filepath.Glob(filepath.Join(p, g))
			for _, m := range ms {
				r, err1 := filepath.Rel(p, m)
				fi, err2 := os.Stat(m)
				if err1 == nil && err2 == nil && !fi.IsDir() {
					fs = append(fs, file{Abs: m, Rel: r})
				}
			}
		}
	}
	return fs
}

func runBenchmarks(files []file, codecs []string, formats []Format, tests []Test, levels, sizes []int) {
	for _, f := range formats {
		// Get lists of encoders and decoders that exist.
		var encs, decs []string
		for _, c := range codecs {
			if _, ok := encoders[f][c]; ok {
				encs = append(encs, c)
			}
		}
		for _, c := range codecs {
			if _, ok := decoders[f][c]; ok {
				decs = append(decs, c)
			}
		}

		for _, t := range tests {
			var results [][]Result
			var names, codecs []string
			var title, suffix string

			// Check that we can actually do this
			fmt.Printf("BENCHMARK: %s:%s\n", enumToFmt[f], enumToTest[t])
			if len(encs) == 0 {
				fmt.Println("\tSKIP: There are no encoders available.")
				fmt.Println("")
				continue
			}
			if len(decs) == 0 && t == TestDecodeRate {
				fmt.Println("\tSKIP: There are no decoders available.")
				fmt.Println("")
				continue
			}

			// Progress ticker.
			var cnt int
			tick := func() {
				total := len(codecs) * len(files) * len(levels) * len(sizes)
				pct := 100.0 * float64(cnt) / float64(total)
				fmt.Printf("\t[%6.2f%%] %d of %d\r", pct, cnt, total)
				cnt++
			}

			// Perform the  This may take some time.
			switch t {
			case TestEncodeRate:
				codecs, title, suffix = encs, "MB/s", ""
				results, names = BenchmarkEncoderSuite(f, encs, files, levels, sizes, tick)
			case TestDecodeRate:
				ref := getReferenceEncoder(f)
				codecs, title, suffix = decs, "MB/s", ""
				results, names = BenchmarkDecoderSuite(f, decs, files, levels, sizes, ref, tick)
			case TestCompressRatio:
				codecs, title, suffix = encs, "ratio", "x"
				results, names = BenchmarkRatioSuite(f, encs, files, levels, sizes, tick)
			default:
				panic("unknown test")
			}

			// Print all of the results.
			printResults(results, names, codecs, title, suffix)
			fmt.Println()
		}
		fmt.Println()
	}
}

// The decompression speed benchmark works by decompressing some pre-compressed
// data. In order for the benchmarks to be consistent, the same encoder should
// be used to generate the pre-compressed data for all the trials.
//
// encRefs defines the priority order for which encoders to choose first as the
// reference compressor. If no compressor is found for any of the listed codecs,
// then a random encoder will be chosen.
var encRefs = []string{"std", "cgo", "ds"}

func getReferenceEncoder(f Format) Encoder {
	for _, c := range encRefs {
		if enc, ok := encoders[f][c]; ok {
			return enc // Choose by priority
		}
	}
	for _, enc := range encoders[f] {
		return enc // Choose any random encoder
	}
	return nil // There are no encoders
}

func printResults(results [][]Result, names, codecs []string, title, suffix string) {
	// Allocate result table.
	cells := make([][]string, 1+len(names))
	for i := range cells {
		cells[i] = make([]string, 1+2*len(codecs))
	}

	// Label the first row.
	cells[0][0] = "benchmark"
	for i, c := range codecs {
		cells[0][1+2*i] = c + " " + title
		cells[0][2+2*i] = "delta"
	}

	// Insert all rows.
	for j, row := range results {
		cells[1+j][0] = names[j]
		for i, r := range row {
			if r.R != 0 && !math.IsNaN(r.R) && !math.IsInf(r.R, 0) {
				cells[1+j][1+2*i] = fmt.Sprintf("%.2f", r.R) + suffix
			}
			if r.D != 0 && !math.IsNaN(r.D) && !math.IsInf(r.D, 0) {
				cells[1+j][2+2*i] = fmt.Sprintf("%.2f", r.D) + "x"
			}
		}
	}

	// Compute the maximum lengths.
	maxLens := make([]int, 1+2*len(codecs))
	for _, row := range cells {
		for i, s := range row {
			if maxLens[i] < len(s) {
				maxLens[i] = len(s)
			}
		}
	}

	// Print padded versions of all cells.
	for _, row := range cells {
		fmt.Print("\t")
		for i, s := range row {
			switch {
			case i == 0: // Column 0
				row[i] = s + strings.Repeat(" ", maxLens[i]-len(s))
			case i%2 == 1: // Column 1, 3, 5, 7, ...
				row[i] = strings.Repeat(" ", 6+maxLens[i]-len(s)) + s
			case i%2 == 0: // Column 2, 4, 6, 8, ...
				row[i] = strings.Repeat(" ", 2+maxLens[i]-len(s)) + s
			}
			fmt.Print(row[i])
		}
		fmt.Println()
	}
}
