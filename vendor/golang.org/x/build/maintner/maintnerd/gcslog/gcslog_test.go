// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gcslog

import (
	"context"
	"testing"
	"time"

	"golang.org/x/build/maintner/maintpb"
)

func TestGCSLogWakeup_Timeout(t *testing.T) {
	testGCSLogWakeup(t, false)
}

func TestGCSLogWakeup_Activity(t *testing.T) {
	testGCSLogWakeup(t, true)
}

func testGCSLogWakeup(t *testing.T, activity bool) {
	gl := newGCSLogBase()
	waitc := make(chan bool, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	go func() {
		waitc <- gl.waitSizeNot(ctx, 0)
	}()
	if activity {
		if err := gl.Log(new(maintpb.Mutation)); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case got := <-waitc:
		if got != activity {
			t.Errorf("changed = %v; want %v", got, activity)
		}
	case <-time.After(2 * time.Second):
		t.Errorf("timeout")
	}
}
