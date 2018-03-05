package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestMatchPattern(t *testing.T) {
	// Test cases from $GOROOT/src/cmd/go/match_test.go.
	cases := []struct {
		pat  string
		path string
		want bool
	}{
		{"...", "foo", true},
		{"net", "net", true},
		{"net", "net/http", false},
		{"net/http", "net", false},
		{"net/http", "net/http", true},
		{"net...", "netchan", true},
		{"net...", "net", true},
		{"net...", "net/http", true},
		{"net...", "not/http", false},
		{"net/...", "netchan", false},
		{"net/...", "net", true},
		{"net/...", "net/http", true},
		{"net/...", "not/http", false},
	}
	for _, test := range cases {
		ok := matchPattern(test.pat)(test.path)
		if ok != test.want {
			t.Errorf("matchPackages(%q)(%q) = %v want %v", test.pat, test.path, ok, test.want)
		}
	}
}

func TestSubPath(t *testing.T) {
	cases := []struct {
		sub  string
		dir  string
		want bool
	}{
		//Basic
		{`/Users/emuller/go/src/github.com/tools/godep`, `/Users/emuller/go`, true},
		//Case insensitive filesystem used in dir
		{`/Users/emuller/go/src/github.com/tools/godep`, `/Users/Emuller/go`, true},
		{`/Users/emuller/go/Src/github.com/tools/godep`, `/Users/Emuller/go`, true},
		//spaces
		{`/Users/e muller/go/Src/github.com/tools/godep`, `/Users/E muller/go`, true},
		// ()
		{`/Users/e muller/(Personal)/go/Src/github.com/tools/godep`, `/Users/E muller/(Personal)/go`, true},
		//Not even close, but same length
		{`/foo`, `/bar`, false},
		// Same, so not sub path (same path)
		{`/foo`, `/foo`, false},
		// Windows with different cases
		{`c:\foo\bar`, `C:\foo`, true},
	}

	for _, test := range cases {
		ok := subPath(test.sub, test.dir)
		if ok != test.want {
			t.Errorf("subdir(%s,%s) = %v want %v", test.sub, test.dir, ok, test.want)
		}
	}
}

func TestIsSameOrNewer(t *testing.T) {
	cases := []struct {
		base  string
		check string
		want  bool
	}{
		{`go1.6`, `go1.6`, true},
		{`go1.5`, `go1.6`, true},
		{`go1.7`, `go1.6`, false},
		{`go1.6`, `devel-8f48efb`, true}, // devel versions are always never
	}

	for _, test := range cases {
		ok := isSameOrNewer(test.base, test.check)
		if ok != test.want {
			t.Errorf("isSameOrNewer(%s,%s) = %v want %v", test.base, test.check, ok, test.want)
		}
	}
}

func TestDetermineVersion(t *testing.T) {
	cases := []struct {
		v         string
		go15ve    string
		vendorDir []string
		want      bool
	}{
		{"go1.5", "", nil, false},
		{"go1.5", "1", nil, true},
		{"go1.5", "1", []string{"Godeps", "_workspace"}, false},
		{"go1.5", "0", nil, false},
		{"go1.6", "", nil, true},
		{"go1.6", "1", nil, true},
		{"go1.6", "1", []string{"Godeps", "_workspace"}, false},
		{"go1.6", "0", nil, false},
		{"devel", "", nil, true},
		{"devel-12345", "", nil, true},
		{"devel", "1", nil, true},
		{"devel-12345", "1", nil, true},
		{"devel", "1", []string{"Godeps", "_workspace"}, false},
		{"devel-12345", "1", []string{"Godeps", "_workspace"}, false},
		{"devel", "0", nil, true},
		{"devel-12345", "0", nil, true},
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Chdir(wd)
	}()

	ove := os.Getenv("GO15VENDOREXPERIMENT")
	defer func() {
		os.Setenv("GO15VENDOREXPERIMENT", ove)
	}()

	for i, test := range cases {
		os.Setenv("GO15VENDOREXPERIMENT", test.go15ve)
		tdir, err := ioutil.TempDir("", "godeptest")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tdir)
		os.Chdir(tdir)

		if len(test.vendorDir) > 0 {
			md := tdir
			for _, vd := range test.vendorDir {
				md = filepath.Join(md, vd)
				if err := os.Mkdir(md, os.ModePerm); err != nil {
					t.Fatal(err)
				}
			}
		}

		if e := determineVendor(test.v); e != test.want {
			t.Errorf("%d GO15VENDOREXPERIMENT=%s determineVendor(%s) == %t, but wanted %t\n", i, test.go15ve, test.v, e, test.want)
		}
	}
}
