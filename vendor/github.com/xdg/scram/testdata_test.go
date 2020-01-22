// Copyright 2018 by David A. Golden. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package scram

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

type TestCase struct {
	Label        string
	Digest       string
	User         string
	Pass         string
	AuthzID      string
	SkipSASLprep bool
	Salt64       string
	Iters        int
	ClientNonce  string
	ServerNonce  string
	Valid        bool
	Steps        []string
}

type testStep struct {
	Input   string
	Expect  string
	IsError bool
}

func getHGF(s string) (HashGeneratorFcn, error) {
	switch s {
	case "SHA-1":
		return SHA1, nil
	case "SHA-256":
		return SHA256, nil
	default:
		panic(fmt.Sprintf("Unknown hash function '%s'", s))
	}
}

func decodeFile(s string) (TestCase, error) {
	var tc TestCase

	data, err := ioutil.ReadFile(s)
	if err != nil {
		return tc, err
	}

	err = json.Unmarshal(data, &tc)
	if err != nil {
		return tc, fmt.Errorf("error unmarshaling '%s': %v", s, err)
	}

	return tc, nil
}

func getTestFiles(dir string) ([]string, error) {
	subdir := filepath.Join("testdata", dir)
	files, err := ioutil.ReadDir(subdir)
	if err != nil {
		return nil, err
	}

	filenames := make([]string, len(files))
	for i, v := range files {
		filenames[i] = filepath.Join(subdir, v.Name())
	}

	return filenames, nil
}

func getTestData(dirs ...string) ([]TestCase, error) {
	var err error
	filenames := make([]string, 0)
	for _, v := range dirs {
		names, err := getTestFiles(v)
		if err != nil {
			return nil, err
		}
		filenames = append(filenames, names...)
	}

	cases := make([]TestCase, len(filenames))
	for i, v := range filenames {
		cases[i], err = decodeFile(v)
		if err != nil {
			return nil, err
		}
	}

	return cases, nil
}

// Even steps are client messages; odd steps are server responses.
func clientSteps(c TestCase) []testStep {
	n := len(c.Steps)

	// Test case requires at least two steps: the first client step
	// (which cannot fail) and the first server response -- after which
	// an error would prevent further client steps.
	if n < 2 {
		panic("Incomplete conversation for this test case")
	}

	// First step needs empty input.
	steps := []testStep{{Input: "", Expect: c.Steps[0]}}

	// From i==1 until end, construct conversations from pairs of steps.  We
	// know that (n >= 2).  If the last pair is incomplete (no client Expect)
	// that indicates error.
	last := n - 1
	for i := 1; i <= last; i += 2 {
		steps = append(steps, assembleStep(c, i, last))
	}

	return steps
}

// Even steps are client messages; odd steps are server responses.
func serverSteps(c TestCase) []testStep {
	n := len(c.Steps)

	// Test case requires at least one step: the first client step
	// after which an error would prevent further server steps.
	if n == 0 {
		panic("Incomplete conversation for this test case")
	}

	steps := make([]testStep, 0, 1)

	// From i==0 until end, construct conversations from pairs of steps.  We
	// know that (n >= 1).  If the last pair is incomplete (no server Expect)
	// that indicates error.
	last := n - 1
	for i := 0; i < last; i += 2 {
		ts := assembleStep(c, i, last)
		steps = append(steps, ts)
	}

	return steps
}

func assembleStep(c TestCase, i int, last int) testStep {
	ts := testStep{Input: c.Steps[i]}
	if i == last {
		ts.IsError = true
	} else {
		ts.Expect = c.Steps[i+1]
		if strings.HasPrefix(ts.Expect, "e=") {
			ts.IsError = true
		}
	}
	return ts
}
