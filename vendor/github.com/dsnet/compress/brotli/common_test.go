// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

import (
	"bytes"
	"hash/crc32"
	"testing"
)

func TestTableCRC(t *testing.T) {
	// Convert transformLUT to byte array according to Appendix B of the RFC.
	var transformBuf bytes.Buffer
	for _, t := range transformLUT {
		transformBuf.WriteString(t.prefix + "\x00")
		transformBuf.WriteByte(byte(t.transform))
		transformBuf.WriteString(t.suffix + "\x00")
	}

	vectors := []struct {
		crc uint32
		buf []byte
	}{
		{crc: 0x5136cb04, buf: dictLUT[:]},
		{crc: 0x8e91efb7, buf: contextLUT0[:]},
		{crc: 0xd01a32f4, buf: contextLUT1[:]},
		{crc: 0x0dd7a0d6, buf: contextLUT2[:]},
		{crc: 0x3d965f81, buf: transformBuf.Bytes()},
	}

	for i, v := range vectors {
		crc := crc32.ChecksumIEEE(v.buf)
		if crc != v.crc {
			t.Errorf("test %d, CRC-32 mismatch: got %08x, want %08x", i, crc, v.crc)
		}
	}
}

// This package relies on dynamic generation of LUTs to reduce the static
// binary size. This benchmark attempts to measure the startup cost of init.
// This benchmark is not thread-safe; so do not run it in parallel with other
// tests or benchmarks!
func BenchmarkInit(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		initLUTs()
	}
}
