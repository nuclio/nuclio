// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"net/http/httptest"
	"testing"
)

func TestHandleBuilders(t *testing.T) {
	rec := httptest.NewRecorder()
	handleBuilders(rec, httptest.NewRequest("GET", "/builders", nil))
	res := rec.Result()
	if res.StatusCode != 200 {
		t.Fatalf("Want 200 OK. Got status: %v, %s", res.Status, rec.Body.Bytes())
	}
	t.Logf("Got: %s", rec.Body.Bytes())
}
