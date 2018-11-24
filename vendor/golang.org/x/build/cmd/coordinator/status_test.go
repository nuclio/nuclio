// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"testing"
	"time"
)

var durationTests = []struct {
	in   time.Duration
	want string
}{
	{10*time.Second + 555*time.Millisecond, "10.6s"},
	{10*time.Second + 500*time.Millisecond, "10.5s"},
	{10*time.Second + 499*time.Millisecond, "10.5s"},
	{10*time.Second + 401*time.Millisecond, "10.4s"},
	{9*time.Second + 401*time.Millisecond, "9.4s"},
	{9*time.Second + 456*time.Millisecond, "9.46s"},
	{9*time.Second + 445*time.Millisecond, "9.45s"},
	{1 * time.Second, "1s"},
	{859*time.Millisecond + 445*time.Microsecond, "859.4ms"},
	{859*time.Millisecond + 460*time.Microsecond, "859.5ms"},
}

func TestFriendlyDuration(t *testing.T) {
	for _, tt := range durationTests {
		got := friendlyDuration(tt.in)
		if got != tt.want {
			t.Errorf("friendlyDuration(%v): got %s, want %s", tt.in, got, tt.want)
		}
	}
}
