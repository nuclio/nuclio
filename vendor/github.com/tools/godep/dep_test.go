package main

import "testing"

func TestTrimGoVersion(t *testing.T) {
	var cases = []struct {
		in, out string
		err     bool
	}{
		{in: "go1.5", out: "go1.5", err: false},
		{in: "go1.5beta1", out: "go1.5", err: false},
		{in: "go1.6rc1", out: "go1.6", err: false},
		{in: "go1.5.1", out: "go1.5", err: false},
		{in: "devel", out: "devel", err: false},
		{in: "devel+15f7a66", out: "devel-15f7a66", err: false},
		{in: "devel-15f7a66", out: "devel-15f7a66", err: false},
		{in: "boom", out: "", err: true},
	}

	for _, c := range cases {
		mv, err := trimGoVersion(c.in)
		if err != nil && !c.err {
			t.Errorf("Unexpected error: %s", err)
		}
		if mv != c.out {
			t.Errorf("Expected trimGoVersion(%s) == '%s', but got '%s'", c.in, c.out, mv)
		}
	}
}

func TestGoVersion(t *testing.T) {
	var cases = []struct {
		o, r string
		err  bool
	}{
		{o: "go version go1.6.2 darwin/amd64", r: "go1.6", err: false},
		{o: "go version go1.6 darwin/amd64", r: "go1.6", err: false},
		{o: "go version go1.6.2 linux/amd64", r: "go1.6", err: false},
		{o: "go version devel +da6205b Wed Apr 13 17:22:38 2016 +0000 darwin/amd64", r: "devel-da6205b", err: false},
	}

	for _, c := range cases {
		goVersionTestOutput = c.o
		v, err := goVersion()
		if err != nil && !c.err {
			t.Errorf("Unexpected error: %s", err)
		}
		if v != c.r {
			t.Errorf("Expected goVersion() == '%s', but got '%s'", c.r, v)
		}
		goVersionTestOutput = ""
	}
}
