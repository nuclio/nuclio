// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package main

import (
	"fmt"
	"go/build"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/dsnet/golib/unitconv"
)

// Section: Format and Test enumerations

type Format int

const (
	FormatFlate Format = iota
	FormatBrotli
	FormatBZ2
	FormatLZMA2
	FormatZstd
)

type Test int

const (
	TestEncodeRate Test = iota
	TestDecodeRate
	TestCompressRatio
)

var (
	fmtToEnum = map[string]Format{
		"fl":   FormatFlate,
		"br":   FormatBrotli,
		"bz2":  FormatBZ2,
		"lzma": FormatLZMA2,
		"zstd": FormatZstd,
	}
	enumToFmt = map[Format]string{
		FormatFlate:  "fl",
		FormatBrotli: "br",
		FormatBZ2:    "bz2",
		FormatLZMA2:  "lzma",
		FormatZstd:   "zstd",
	}
	testToEnum = map[string]Test{
		"encRate": TestEncodeRate,
		"decRate": TestDecodeRate,
		"ratio":   TestCompressRatio,
	}
	enumToTest = map[Test]string{
		TestEncodeRate:    "encRate",
		TestDecodeRate:    "decRate",
		TestCompressRatio: "ratio",
	}
)

// Section: Encoders and Decoders
//
// In order for new encoders and decoders (also called codecs) to be added,
// the RegisterEncoder and RegisterDecoder functions can be used to inform
// the tool about the existence of a given implementation for some format.
// Source files (protected by build tags) can use an init function to register
// a codec for testing and benchmarking purposes.

type Encoder func(io.Writer, int) io.WriteCloser
type Decoder func(io.Reader) io.ReadCloser

var (
	encoders map[Format]map[string]Encoder
	decoders map[Format]map[string]Decoder
)

func RegisterEncoder(ft Format, name string, enc Encoder) {
	if encoders == nil {
		encoders = make(map[Format]map[string]Encoder)
	}
	if encoders[ft] == nil {
		encoders[ft] = make(map[string]Encoder)
	}
	encoders[ft][name] = enc
}

func RegisterDecoder(ft Format, name string, dec Decoder) {
	if decoders == nil {
		decoders = make(map[Format]map[string]Decoder)
	}
	if decoders[ft] == nil {
		decoders[ft] = make(map[string]Decoder)
	}
	decoders[ft][name] = dec
}

// Section: Configuration variables
//
// These are list of global configuration variables that can be set to alter
// the behavior of tests and benchmarks. Setting of these variables is done
// by setting flags on the command line.

var (
	formats varFormats
	tests   varTests
	codecs  varStrings
	paths   varStrings
	globs   varStrings
	levels  varInts
	sizes   varInts
)

// setDefaults configures the top-level parameters with default values.
// This function method must be called after all init functions have executed
// since they register the various codecs.
func setDefaults() {
	formats = defaultFormats()
	tests = defaultTests()
	codecs = defaultCodecs()
	paths = defaultPaths()
	globs = []string{"*.txt", "*.bin"}
	levels = []int{1, 6, 9}
	sizes = []int{1e4, 1e5, 1e6}
}

func defaultFormats() []Format {
	m := make(map[Format]bool)
	for k := range encoders {
		m[k] = true
	}
	for k := range decoders {
		m[k] = true
	}
	var d []int
	for k := range m {
		d = append(d, int(k))
	}
	sort.Ints(d)
	var fs []Format
	for _, f := range d {
		fs = append(fs, Format(f))
	}
	return fs
}

func defaultTests() []Test {
	var d []int
	for k := range enumToTest {
		d = append(d, int(k))
	}
	sort.Ints(d)
	var ts []Test
	for _, t := range d {
		ts = append(ts, Test(t))
	}
	return ts
}

func defaultCodecs() []string {
	m := make(map[string]bool)
	for _, v := range encoders {
		for k := range v {
			m[k] = true
		}
	}
	for _, v := range decoders {
		for k := range v {
			m[k] = true
		}
	}
	hasDS := m["std"]
	delete(m, "std")
	var cs []string
	for k := range m {
		cs = append(cs, k)
	}
	sort.Strings(cs)
	if hasDS {
		cs = append([]string{"std"}, cs...) // Ensure "std" always appears first
	}
	return cs
}

func defaultPaths() []string {
	const testdataPkg = "github.com/dsnet/compress/testdata"
	pkg, err := build.Import(testdataPkg, "", build.FindOnly)
	if err != nil {
		return nil
	}
	return []string{pkg.Dir}
}

var reDelim = regexp.MustCompile("[,:]")

type (
	varFormats []Format
	varTests   []Test
	varStrings []string
	varInts    []int
)

func (fs *varFormats) String() string {
	var ss []string
	for _, f := range *fs {
		ss = append(ss, enumToFmt[f])
	}
	return strings.Join(ss, ",")
}
func (fs *varFormats) Set(ss string) error {
	*fs = nil
	for _, s := range reDelim.Split(ss, -1) {
		f, ok := fmtToEnum[s]
		if !ok {
			return fmt.Errorf("unknown format: %s", s)
		}
		*fs = append(*fs, f)
	}
	return nil
}

func (ts *varTests) String() string {
	var ss []string
	for _, t := range *ts {
		ss = append(ss, enumToTest[t])
	}
	return strings.Join(ss, ",")
}
func (ts *varTests) Set(ss string) error {
	*ts = nil
	for _, s := range reDelim.Split(ss, -1) {
		t, ok := testToEnum[s]
		if !ok {
			return fmt.Errorf("unknown test: %s", s)
		}
		*ts = append(*ts, t)
	}
	return nil
}

func (vs *varStrings) String() string {
	return strings.Join(*vs, ",")
}
func (vs *varStrings) Set(ss string) error {
	*vs = reDelim.Split(ss, -1)
	return nil
}

func (ds *varInts) String() string {
	var ss []string
	for _, d := range *ds {
		ss = append(ss, intName(d))
	}
	return strings.Join(ss, ",")
}
func (ds *varInts) Set(ss string) error {
	*ds = nil
	for _, s := range reDelim.Split(ss, -1) {
		d, err := unitconv.ParsePrefix(s, unitconv.AutoParse)
		if err != nil {
			return err
		}
		*ds = append(*ds, int(d))
	}
	return nil
}

// intName returns a shorter representation of the input integer.
// It uses scientific notation for exact powers of 10.
// It uses SI suffixes for powers of 1024.
// If the number is small enough, it will be printed as is.
func intName(n int) string {
	switch n {
	case 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10, 1e11, 1e12:
		s := fmt.Sprintf("%e", float64(n))
		re := regexp.MustCompile("\\.0*e\\+0*")
		return re.ReplaceAllString(s, "e")
	default:
		s := unitconv.FormatPrefix(float64(n), unitconv.Base1024, 2)
		return strings.Replace(s, ".00", "", -1)
	}
}
