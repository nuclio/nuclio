// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package xflate

import (
	"bytes"
	"compress/flate"
	"io/ioutil"
	"testing"

	"github.com/dsnet/compress/internal/testutil"
)

func TestWriter(t *testing.T) {
	br := bytes.Repeat
	dh := testutil.MustDecodeHex

	vectors := []struct {
		desc   string        // Test description
		conf   *WriterConfig // Input WriterConfig
		input  []interface{} // Test input tokens (either flush mode or input string)
		output []byte        // Expected output string
	}{{
		desc:  "empty stream",
		input: []interface{}{},
		output: dh("" +
			"0d008705000048c82a51e8ff37dbf1", // Footer
		),
	}, {
		desc:  "empty stream with empty chunk",
		input: []interface{}{FlushSync},
		output: dh("" +
			"000000ffff000000ffff" + // Chunk0
			"34c086050020916cb2a50bd20369da192deaff3bda05f8" + // Index0
			"1dc08605002021ab44219b4aff7fd6de3bf8", // Footer
		),
	}, {
		desc:  "empty stream with empty index",
		input: []interface{}{FlushIndex},
		output: dh("" +
			"04c086050020191d53a1a508c9e8ff5bda7bf8" + // Index0
			"15c08605002021ab44219ba2ff2f6bef5df8", // Footer
		),
	}, {
		desc:  "empty stream with multiple empty chunks",
		input: []interface{}{FlushFull, FlushFull, FlushFull},
		output: dh("" +
			"000000ffff" + // Chunk0
			"000000ffff" + // Chunk1
			"000000ffff" + // Chunk2
			"148086058044655366e3817441ba205d504a83348c445ddcde7b6ffc" + // Index0
			"15c08605002021ab44a103aaff2f6bef5df8", // Footer
		),
	}, {
		desc:  "empty stream with multiple empty indexes",
		input: []interface{}{FlushIndex, FlushIndex, FlushIndex},
		output: dh("" +
			"04c086050020191d53a1a508c9e8ff5bda7bf8" + // Index0
			"3cc08605002019293a24a55464a585faff9bf600f8" + // Index1
			"04c08605002019493a2494d050560afd7f4c7bfb" + // Index2
			"25008705000048c82a51e880f4ff834df0", // Footer
		),
	}, {
		desc:  "3k zeros, 1KiB chunks",
		conf:  &WriterConfig{ChunkSize: 1 << 10},
		input: []interface{}{br([]byte{0}, 3000)},
		output: dh("" +
			"621805a360148c5800000000ffff" + // Chunk0
			"621805a360148c5800000000ffff" + // Chunk1
			"621805a360140c3900000000ffff" + // Chunk2
			"1c8086058044642b3bc9aa3464540784acea809055d99586dd5492446555a7b607fc" + // Index0
			"0d008705000048c82a51c81ea1ff0f6cf2", // Footer
		),
	}, {
		desc: "quick brown fox - manual chunking/indexing",
		input: []interface{}{
			"the quick", FlushSync, " brown fox", FlushFull, FlushFull, " jumped over the", FlushIndex, " lazy dog",
		},
		output: dh("" +
			"2ac94855282ccd4cce06000000ffff52482aca2fcf5348cbaf00000000ffff" + // Chunk0
			"000000ffff" + // Chunk1
			"52c82acd2d484d51c82f4b2d5228c94805000000ffff" + // Chunk2
			"2480860580446553762a0ad14211d207253b234546a1528ad4d3edbd0bfc" + // Index0
			"52c849acaa5448c94f07000000ffff" + // Chunk3
			"2c8086058044a281ec8611190d23b21221ca0851fdafbdf7de05fc" + // Index1
			"1dc08605002021ab44219b52ff7fd6de3bf8", // Footer
		),
	}, {
		desc:  "quick brown fox - automatic chunking/indexing",
		conf:  &WriterConfig{ChunkSize: 4, IndexSize: 3},
		input: []interface{}{"the quick brown fox jumped over the lazy dog"},
		output: dh("" +
			"2ac9485500000000ffff" + // Chunk0
			"2a2ccd4c06000000ffff" + // Chunk1
			"ca56482a02000000ffff" + // Chunk2
			"2c8086058044655376c32a2b9999c9cc4c665691d04ea5a474747bef01fc" + // Index0
			"ca2fcf5300000000ffff" + // Chunk3
			"4acbaf5000000000ffff" + // Chunk4
			"ca2acd2d00000000ffff" + // Chunk5
			"04808605804445036537acb2929999cccc6466cb48112a45a193db7beffc" + // Index1
			"4a4d51c807000000ffff" + // Chunk6
			"2a4b2d5200000000ffff" + // Chunk7
			"2ac9485500000000ffff" + // Chunk8
			"04808605804445036537acb2929999cccc6466cb48112a45a193db7beffc" + // Index2
			"ca49acaa04000000ffff" + // Chunk9
			"5248c94f07000000ffff" + // Chunk10
			"148086058084a261644b665632339399d9425629a44877b7f7de3bfc" + // Index3
			"15c08605002021ab44a103aaff2f6bef5df8", // Footer
		),
	}}

	for i, v := range vectors {
		// Encode the test input.
		var b, bb bytes.Buffer
		xw, err := NewWriter(&b, v.conf)
		if err != nil {
			t.Errorf("test %d (%s), unexpected error: NewWriter() = %v", i, v.desc, err)
		}
		for _, tok := range v.input {
			switch tok := tok.(type) {
			case string:
				bb.WriteString(tok)
				if _, err := xw.Write([]byte(tok)); err != nil {
					t.Errorf("test %d (%s), unexpected error: Write() = %v", i, v.desc, err)
				}
			case []byte:
				bb.Write(tok)
				if _, err := xw.Write(tok); err != nil {
					t.Errorf("test %d (%s), unexpected error: Write() = %v", i, v.desc, err)
				}
			case FlushMode:
				if err := xw.Flush(tok); err != nil {
					t.Errorf("test %d (%s), unexpected error: Flush() = %v", i, v.desc, err)
				}
			default:
				t.Fatalf("test %d (%s), unknown token: %v", i, v.desc, tok)
			}
		}
		if err := xw.Close(); err != nil {
			t.Errorf("test %d (%s), unexpected error: Close() = %v", i, v.desc, err)
		}
		if got, want, ok := testutil.BytesCompare(b.Bytes(), v.output); !ok {
			t.Errorf("test %d (%s), mismatching bytes:\ngot  %s\nwant %s", i, v.desc, got, want)
		}
		if xw.OutputOffset != int64(b.Len()) {
			t.Errorf("test %d (%s), output offset mismatch: got %d, want %d", i, v.desc, xw.OutputOffset, b.Len())
		}
		if xw.InputOffset != int64(bb.Len()) {
			t.Errorf("test %d (%s), input offset mismatch: got %d, want %d", i, v.desc, xw.InputOffset, bb.Len())
		}

		// Verify that the output stream is DEFLATE compatible.
		rd := bytes.NewReader(b.Bytes())
		fr := flate.NewReader(rd)
		buf, err := ioutil.ReadAll(fr)
		if err != nil {
			t.Errorf("test %d (%s), unexpected error: ReadAll() = %v", i, v.desc, err)
		}
		if got, want, ok := testutil.BytesCompare(buf, bb.Bytes()); !ok {
			t.Errorf("test %d (%s), mismatching bytes:\ngot  %s\nwant %s", i, v.desc, got, want)
		}
		if rd.Len() > 0 {
			t.Errorf("test %d (%s), not all bytes consumed: %d > 0", i, v.desc, rd.Len())
		}
	}
}

func TestWriterReset(t *testing.T) {
	// Test bad Writer config.
	xw, err := NewWriter(ioutil.Discard, &WriterConfig{Level: -431})
	if err == nil {
		t.Fatalf("unexpected success: NewWriter()")
	}

	// Test Writer for idempotent Close.
	xw = new(Writer)
	xw.Reset(ioutil.Discard)
	if _, err := xw.Write([]byte("hello, world!")); err != nil {
		t.Fatalf("unexpected error: Write() = %v", err)
	}
	if err := xw.Close(); err != nil {
		t.Fatalf("unexpected error: Close() = %v", err)
	}
	if err := xw.Close(); err != nil {
		t.Fatalf("unexpected error: Close() = %v", err)
	}
	if _, err := xw.Write([]byte("hello, world!")); err != errClosed {
		t.Fatalf("mismatching error: Write() = %v, want %v", err, errClosed)
	}
}

// BenchmarkWriter benchmarks the overhead of the XFLATE format over DEFLATE.
// Thus, it intentionally uses a very small chunk size with no compression.
func BenchmarkWriter(b *testing.B) {
	twain := testutil.MustLoadFile("../testdata/twain.txt")
	bb := bytes.NewBuffer(make([]byte, 0, 2*len(twain)))
	xw, _ := NewWriter(nil, &WriterConfig{Level: NoCompression, ChunkSize: 1 << 10})

	b.ReportAllocs()
	b.SetBytes(int64(len(twain)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		bb.Reset()
		xw.Reset(bb)
		if _, err := xw.Write(twain); err != nil {
			b.Fatalf("unexpected error: Write() = %v", err)
		}
		if err := xw.Close(); err != nil {
			b.Fatalf("unexpected error: Close() = %v", err)
		}
	}
}
