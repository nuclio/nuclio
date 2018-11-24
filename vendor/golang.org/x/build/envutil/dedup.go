// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package envutil provides utilities for working with environment variables.
package envutil

import "strings"

// Dedup returns a copy of env with any duplicates removed, in favor of
// later values.
// Items are expected to be on the normal environment "key=value" form.
// If caseInsensitive is true, the case of keys is ignored.
func Dedup(caseInsensitive bool, env []string) []string {
	out := make([]string, 0, len(env))
	saw := map[string]int{} // to index in the array
	for _, kv := range env {
		eq := strings.Index(kv, "=")
		if eq < 1 {
			out = append(out, kv)
			continue
		}
		k := kv[:eq]
		if caseInsensitive {
			k = strings.ToLower(k)
		}
		if dupIdx, isDup := saw[k]; isDup {
			out[dupIdx] = kv
		} else {
			saw[k] = len(out)
			out = append(out, kv)
		}
	}
	return out
}
