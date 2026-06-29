//go:build test_unit

/*
Copyright 2023 The Nuclio Authors.

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

package resourcescaler

import (
	"testing"
	"time"
)

func TestParseReadinessPollInterval(t *testing.T) {
	for _, testCase := range []struct {
		value    string
		expected time.Duration
	}{
		{value: "", expected: defaultReadinessPollInterval},
		{value: "500ms", expected: 500 * time.Millisecond},
		{value: "0s", expected: defaultReadinessPollInterval},
		{value: "-1s", expected: defaultReadinessPollInterval},
		{value: "garbage", expected: defaultReadinessPollInterval},
	} {
		if got := parseReadinessPollInterval(testCase.value); got != testCase.expected {
			t.Errorf("parseReadinessPollInterval(%q) = %v, want %v", testCase.value, got, testCase.expected)
		}
	}
}
