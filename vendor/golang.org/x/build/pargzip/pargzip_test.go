// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pargzip

import (
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"
	"time"
)

func TestWriter(t *testing.T) {
	var in bytes.Buffer
	big := strings.Repeat("a", 4<<10)
	for in.Len() < 20<<20 {
		for i := 0; i < 256; i++ {
			in.WriteByte(byte(i))
			in.WriteString(big)
		}
	}
	t.Logf("input size = %v", in.Len())
	var zbuf bytes.Buffer
	zw := NewWriter(&zbuf)
	zw.ChunkSize = 1 << 20
	zw.Parallel = 4
	t0 := time.Now()
	if n, err := io.Copy(zw, bytes.NewReader(in.Bytes())); err != nil {
		t.Fatalf("Copy: %v", err)
	} else {
		t.Logf("Copied %d bytes", n)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	td := time.Since(t0)
	t.Logf("Compressed size: %v (%0.2f%%) in %v", zbuf.Len(), float64(zbuf.Len())/float64(in.Len())*100, td)

	var back bytes.Buffer
	zr, _ := gzip.NewReader(bytes.NewReader(zbuf.Bytes()))
	if _, err := io.Copy(&back, zr); err != nil {
		t.Fatalf("uncompress Copy: %v", err)
	}
	if !bytes.Equal(in.Bytes(), back.Bytes()) {
		t.Error("decompression failed.")
	}
	t.Logf("correctly read back %d bytes", back.Len())
}
