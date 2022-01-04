//go:build test_unit

/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package compare

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

var (
	testCases = []struct {
		left   interface{}
		right  interface{}
		result bool
	}{
		// Basic types
		{nil, nil, true},
		{nil, 1, false},
		{1, 1, true},
		{"one", "one", true},
		{"one", "ones", false},

		// Slices
		{[]string{}, []string{}, true},
		{[]string{}, []string{"a"}, false},
		{[]string{"a", "a"}, []string{"a"}, false},
		{[]string{"a"}, []string{"a"}, true},
		{[]string{"b", "a"}, []string{"a", "b"}, true},
		{[][]string{{"b", "a"}, {"c"}}, [][]string{{"c"}, {"a", "b"}}, true},
		{[][]string{{"b", "a", "c"}, {"c"}}, [][]string{{"c"}, {"a", "b"}}, false},

		// Arrays
		{[][2]string{{"b", "a"}, {"c", "d"}}, [][2]string{{"c", "d"}, {"a", "b"}}, true},
		{[][3]string{{"b", "a", "c"}}, [][2]string{{"d", "c"}, {"a", "b"}}, false},

		// Maps
		{map[int]int{1: 1}, map[int]int{1: 1}, true},
		{map[int]int{1: 2}, map[int]int{1: 1}, false},
		{map[int]float32{1: 1}, map[int]int{1: 1}, false}, // different type
		{map[int][]int{1: {1, 2, 3}}, map[int][]int{1: {2, 3, 1}}, true},
		{map[int][]int{1: {1, 2}}, map[int][]int{1: {2, 3}}, false},
	}
)

type CompareTestSuite struct {
	suite.Suite
}

func (suite *CompareTestSuite) TestCases() {
	// TODO: Find out how to make testify work with t.Run (currently panics on error)
	for _, testCase := range testCases {
		result := NoOrder(testCase.left, testCase.right)
		suite.Require().Equalf(testCase.result, result, "%v <-> %v", testCase.right, testCase.left)
	}
}

func TestCompare(t *testing.T) {
	suite.Run(t, new(CompareTestSuite))
}
