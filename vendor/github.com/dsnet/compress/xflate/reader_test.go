// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package xflate

import (
	"bytes"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"testing"

	"github.com/dsnet/compress/internal/errors"
	"github.com/dsnet/compress/internal/testutil"
)

type countReadSeeker struct {
	io.ReadSeeker
	N int64
}

func (rs *countReadSeeker) Read(buf []byte) (int, error) {
	n, err := rs.ReadSeeker.Read(buf)
	rs.N += int64(n)
	return n, err
}

func TestReader(t *testing.T) {
	dh := testutil.MustDecodeHex

	errFuncs := map[string]func(error) bool{
		"IsCorrupted": errors.IsCorrupted,
	}
	vectors := []struct {
		desc   string // Description of the test
		input  []byte // Input test string
		output []byte // Expected output string
		errf   string // Name of error checking callback
	}{{
		desc: "empty string",
		errf: "IsCorrupted",
	}, {
		desc: "empty stream",
		input: dh("" +
			"0d008705000048c82a51e8ff37dbf1",
		),
	}, {
		desc: "empty stream with empty chunk",
		input: dh("" +
			"000000ffff000000ffff34c086050020916cb2a50bd20369da192deaff3bda05" +
			"f81dc08605002021ab44219b4aff7fd6de3bf8",
		),
	}, {
		desc: "empty stream with empty index",
		input: dh("" +
			"04c086050020191d53a1a508c9e8ff5bda7bf815c08605002021ab44219ba2ff" +
			"2f6bef5df8",
		),
	}, {
		desc: "empty stream with multiple empty chunks",
		input: dh("" +
			"000000ffff000000ffff000000ffff148086058044655366e3817441ba205d50" +
			"4a83348c445ddcde7b6ffc15c08605002021ab44a103aaff2f6bef5df8",
		),
	}, {
		desc: "empty stream with multiple empty chunks, with final bit",
		input: dh("" +
			"000000ffff010000ffff000000ffff148086058044655366e3817441ba205d50" +
			"4a83348c445ddcde7b6ffc15c08605002021ab44a103aaff2f6bef5df8",
		),
		errf: "IsCorrupted",
	}, {
		desc: "empty stream with multiple empty indexes",
		input: dh("" +
			"04c086050020191d53a1a508c9e8ff5bda7bf83cc08605002019293a24a55464" +
			"a585faff9bf600f804c08605002019493a2494d050560afd7f4c7bfb25008705" +
			"000048c82a51e880f4ff834df0",
		),
	}, {
		desc: "3k zeros, 1KiB chunks",
		input: dh("" +
			"621805a360148c5800000000ffff621805a360148c5800000000ffff621805a3" +
			"60140c3900000000ffff1c8086058044642b3bc9aa3464540784acea809055d9" +
			"9586dd5492446555a7b607fc0d008705000048c82a51c81ea1ff0f6cf2",
		),
		output: make([]byte, 3000),
	}, {
		desc: "quick brown fox - spec example",
		input: dh("" +
			"0ac94855282ccd4cce560028a928bf3c4f212dbf4201a0acd2dc82d41485fcb2" +
			"d42205804a80f2398955950a00000000ffff4ac94f5704000000ffff24808605" +
			"8084b247b60629218a48486656d2b442ca489fb7f7de0bfc3cc08605002019a1" +
			"3aa454548a122ad5fff7b403f815c08605002021ab44219ba4ff2f6bef5df8",
		),
		output: []byte("The quick brown fox jumped over the lazy dog!"),
	}, {
		desc: "quick brown fox - manual chunking/indexing",
		input: dh("" +
			"2ac94855282ccd4cce06000000ffff52482aca2fcf5348cbaf00000000ffff00" +
			"0000ffff52c82acd2d484d51c82f4b2d5228c94805000000ffff248086058044" +
			"6553762a0ad14211d207253b234546a1528ad4d3edbd0bfc52c849acaa5448c9" +
			"4f07000000ffff2c8086058044a281ec8611190d23b21221ca0851fdafbdf7de" +
			"05fc1dc08605002021ab44219b52ff7fd6de3bf8",
		),
		output: []byte("the quick brown fox jumped over the lazy dog"),
	}, {
		desc: "quick brown fox - automatic chunking/indexing",
		input: dh("" +
			"2ac9485500000000ffff2a2ccd4c06000000ffffca56482a02000000ffff2c80" +
			"86058044655376c32a2b9999c9cc4c665691d04ea5a474747bef01fcca2fcf53" +
			"00000000ffff4acbaf5000000000ffffca2acd2d00000000ffff048086058044" +
			"45036537acb2929999cccc6466cb48112a45a193db7beffc4a4d51c807000000" +
			"ffff2a4b2d5200000000ffff2ac9485500000000ffff04808605804445036537" +
			"acb2929999cccc6466cb48112a45a193db7beffcca49acaa04000000ffff5248" +
			"c94f07000000ffff148086058084a261644b665632339399d9425629a44877b7" +
			"f7de3bfc15c08605002021ab44a103aaff2f6bef5df8",
		),
		output: []byte("the quick brown fox jumped over the lazy dog"),
	}, {
		desc: "alphabet",
		input: dh("" +
			"4a4c4a4e494d4bcfc8cccacec9cdcb2f282c2a2e292d2bafa8ac02000000ffff" +
			"048086058044b2e98190b285148a844a0b95a4f7db7bef3dfc15c08605002021" +
			"ab44219ba8ff2f6bef5df8",
		),
		output: []byte("abcdefghijklmnopqrstuvwxyz"),
	}, {
		desc:  "garbage footer",
		input: dh("5174453181b67484bf6de23a608876f8b7f44c77"),
		errf:  "IsCorrupted",
	}, {
		desc:  "corrupt meta footer",
		input: dh("1d008705000048ca2c50e8ff3bdbf0"),
		errf:  "IsCorrupted",
	}, {
		desc:  "trailing meta data in footer",
		input: dh("0d008705000048c82a51e8ff37dbf1deadcafe"),
		errf:  "IsCorrupted",
	}, {
		desc:  "trailing raw data in footer",
		input: dh("25c086050020a9ac12856ec8284229d4ff0fb527f8"),
		errf:  "IsCorrupted",
	}, {
		desc:  "footer using LastMeta",
		input: dh("0c008705000048c82a51e8ff37dbf1"),
		errf:  "IsCorrupted",
	}, {
		desc:  "footer without magic",
		input: dh("1d00870500004864a644eaff3bdbf0"),
		errf:  "IsCorrupted",
	}, {
		desc:  "footer with VLI overflow",
		input: dh("2d80860580944a458a4abb6e6c9fdbde7bef01fc"),
		errf:  "IsCorrupted",
	}, {
		desc: "index using LastStream",
		input: dh("" +
			"05c086050020191d53a1a508c9e8ff5bda7bf815c08605002021ab44219ba2ff" +
			"2f6bef5df8",
		),
		errf: "IsCorrupted",
	}, {
		desc: "index with wrong CRC",
		input: dh("" +
			"2cc086050020191d132551320a51ff9fd2de0bf825008705000048c82a51e880" +
			"f4ff834df0",
		),
		errf: "IsCorrupted",
	}, {
		desc: "corrupt meta index",
		input: dh("" +
			"04c086050020191d53a1a518c9e8ff5bda7bf815c08605002021ab44219ba2ff" +
			"2f6bef5df8",
		),
		errf: "IsCorrupted",
	}, {
		desc: "index with VLI overflow",
		input: dh("" +
			"048086058094e8c6f6de7b531215458a840e6deffc15c08605002021ab44219b" +
			"a4ff2f6bef5df8",
		),
		errf: "IsCorrupted",
	}, {
		desc: "trailing meta data in index",
		input: dh("" +
			"34c086050020291d53a1a508c908a16414a2fe3fa205f81dc08605002021ab44" +
			"219b4aff7fd6de3bf8",
		),
		errf: "IsCorrupted",
	}, {
		desc: "trailing raw data in index",
		input: dh("" +
			"04c086050020191d53a1a508c9e8ff5bda7bf862616405c08605002021ab4421" +
			"7b94febfacbd77f9",
		),
		errf: "IsCorrupted",
	}, {
		desc: "index total size is wrong",
		input: dh("" +
			"000000ffff14c086050020916cb2d505e983840aa12592faff8c76f81dc08605" +
			"002021ab44219b4aff7fd6de3bf8",
		),
		errf: "IsCorrupted",
	}, {
		desc: "index with compressed chunk size of zero",
		input: dh("" +
			"000000ffff04c086050020916cb2e9848e8894a2a441fd7f457bf905c0860500" +
			"2021ab44217b94febfacbd77f9",
		),
		errf: "IsCorrupted",
	}, {
		desc: "index with numeric overflow on sizes",
		input: dh("" +
			"000000ffff000000ffff0c40860552a43db4a53dcf6b97b47724641589a84e69" +
			"efbdf7de7b4ffe1dc08605002021ab44219b54ff7fd6de3bf8",
		),
		errf: "IsCorrupted",
	}, {
		desc: "empty chunk without sync marker",
		input: dh("" +
			"000000ffff020820800004c086050020a1ec919d1e4817a40b421269a3a8ff1f" +
			"68fa2d008705000048c82a51e881faffc126f0",
		),
		errf: "IsCorrupted",
	}, {
		desc: "chunk without sync marker",
		input: dh("" +
			"000000ffff000200fdff486902082080000cc086050020a1ec91193232d30965" +
			"652b2b221125f5ff1eedf805c08605002021ab44217ba4febfacbd77f9",
		),
		output: []byte("Hi"),
		errf:   "IsCorrupted",
	}, {
		desc: "chunk with wrong sizes",
		input: dh("" +
			"000000ffff000200fdff4869000000ffff2c8086058084b2476608d9e98432b2" +
			"15252a958a92eaeef6de7b07fc15c08605002021ab44a103aaff2f6bef5df8",
		),
		output: []byte("Hi"),
		errf:   "IsCorrupted",
	}, {
		desc: "size overflow across multiple indexes",
		input: dh("" +
			"000000ffff0c8086058094b487b6b4ce4b5ae7150d49d124195dd29efc000000" +
			"ffff000000ffff24808605808432cac84e4676ba2059d9914a4a29259a8fb7f7" +
			"de0bfc15c08605002021ab44a103aaff2f6bef5df8",
		),
		errf: "IsCorrupted",
	}, {
		desc: "index back size causes integer overflow",
		input: dh("" +
			"4a4c4a4e494d4bcfc8cccacec9cdcb2f282c2a2e292d2bafa8ac02000000ffff" +
			"048086058044b2e98190b285148a844a0b95a4f7db7bef3dfc4a4c4a4e494d4b" +
			"cfc8cccacec9cdcb2f282c2a2e292d2bafa8ac02000000ffff2c8086058094e8" +
			"bcb4a74ab4538986529284cc3e6def05fc2d008705000048c82a51e881faffc1" +
			"26f0"),
		errf: "IsCorrupted",
	}, {
		desc: "raw chunk with final bit and bad size",
		input: dh("" +
			"010900f6ff0000ffff248086058044b2c98e8cc8888cc828ed9d284afa7fb4f7" +
			"de0bfc05c08605002021ab44217ba4febfacbd77f9",
		),
		output: dh("0000ffff010000ffff"),
		// TODO(dsnet): The Reader incorrectly believes that this is valid.
		// The chunk has a final raw block with a specified size of 9, but only
		// has 4 bytes following it (0000ffff to fool the sync check).
		// Since the decompressor would expect an additional 5 bytes, this is
		// satisfied by the fact that the chunkReader appends the endBlock
		// sequence (010000ffff) to every chunk. This really difficult to fix
		// without low-level details about the DEFLATE stream.
		errf: "", // "IsCorrupted",
	}}

	for i, v := range vectors {
		var xr *Reader
		var err error
		var buf []byte

		xr, err = NewReader(bytes.NewReader(v.input), nil)
		if err != nil {
			goto done
		}

		buf, err = ioutil.ReadAll(xr)
		if err != nil {
			goto done
		}

	done:
		if v.errf != "" && !errFuncs[v.errf](err) {
			t.Errorf("test %d (%s), mismatching error:\ngot %v\nwant %s(err) == true", i, v.desc, err, v.errf)
		} else if v.errf == "" && err != nil {
			t.Errorf("test %d (%s), unexpected error: got %v", i, v.desc, err)
		}
		if got, want, ok := testutil.BytesCompare(buf, v.output); !ok && err == nil {
			t.Errorf("test %d (%s), mismatching output:\ngot  %s\nwant %s", i, v.desc, got, want)
		}
	}
}

func TestReaderReset(t *testing.T) {
	var (
		empty   = testutil.MustDecodeHex("0d008705000048c82a51e8ff37dbf1")
		badSize = testutil.MustDecodeHex("" +
			"4a4c4a4e494d4bcfc8cccacec9cdcb2f282c2a2e292d2bafa8ac02000000ffff" +
			"3c8086058084b2e981acd0203b2b34884a834a2a91d2ededbd7701fc15c08605" +
			"002021ab44a103aaff2f6bef5df8",
		)
		badData = testutil.MustDecodeHex("" +
			"4a4c4a4e494d4bcfc8cccacec9cdcb2f282c2a2e292d2baf000002000000ffff" +
			"048086058044b2e98190b285148a844a0b95a4f7db7bef3dfc15c08605002021" +
			"ab44219ba8ff2f6bef5df8",
		)
	)

	// Test Reader for idempotent Close.
	xr := new(Reader)
	if err := xr.Reset(bytes.NewReader(empty)); err != nil {
		t.Fatalf("unexpected error: Reset() = %v", err)
	}
	buf, err := ioutil.ReadAll(xr)
	if err != nil {
		t.Fatalf("unexpected error: ReadAll() = %v", err)
	}
	if len(buf) > 0 {
		t.Fatalf("unexpected output data: ReadAll() = %q, want nil", buf)
	}
	if err := xr.Close(); err != nil {
		t.Fatalf("unexpected error: Close() = %v", err)
	}
	if err := xr.Close(); err != nil {
		t.Fatalf("unexpected error: Close() = %v", err)
	}
	if _, err := ioutil.ReadAll(xr); err != errClosed {
		t.Fatalf("mismatching error: ReadAll() = %v, want %v", err, errClosed)
	}

	// Test Reset on garbage data.
	rd := bytes.NewReader(append([]byte("garbage"), empty...))
	if err := xr.Reset(rd); !errors.IsCorrupted(err) {
		t.Fatalf("mismatching error: Reset() = %v, want IsCorrupted(err) == true", err)
	}
	if _, err := xr.Seek(0, io.SeekStart); !errors.IsCorrupted(err) {
		t.Fatalf("mismatching error: Seek() = %v, want IsCorrupted(err) == true", err)
	}
	if err := xr.Close(); !errors.IsCorrupted(err) {
		t.Fatalf("mismatching error: Close() = %v, want IsCorrupted(err) == true", err)
	}

	// Test Reset on corrupt data in discard section.
	for i, v := range [][]byte{badData, badSize} {
		if err := xr.Reset(bytes.NewReader(v)); err != nil {
			t.Fatalf("test %d, unexpected error: Reset() = %v", i, err)
		}
		if _, err := xr.Seek(-1, io.SeekEnd); err != nil {
			t.Fatalf("test %d, unexpected error: Seek() = %v", i, err)
		}
		if _, err = ioutil.ReadAll(xr); !errors.IsCorrupted(err) {
			t.Fatalf("test %d, mismatching error: ReadAll() = %v, want IsCorrupted(err) == true", i, err)
		}
	}
}

func TestReaderSeek(t *testing.T) {
	rand := rand.New(rand.NewSource(0))
	twain := testutil.MustLoadFile("../testdata/twain.txt")

	// Generate compressed version of input file.
	var buf bytes.Buffer
	xw, err := NewWriter(&buf, &WriterConfig{ChunkSize: 1 << 10})
	if err != nil {
		t.Fatalf("unexpected error: NewWriter() = %v", err)
	}
	if _, err := xw.Write(twain); err != nil {
		t.Fatalf("unexpected error: Write() = %v", err)
	}
	if err := xw.Close(); err != nil {
		t.Fatalf("unexpected error: Close() = %v", err)
	}

	// Read the compressed file.
	rs := &countReadSeeker{ReadSeeker: bytes.NewReader(buf.Bytes())}
	xr, err := NewReader(rs, nil)
	if err != nil {
		t.Fatalf("unexpected error: NewReader() = %v", err)
	}

	// As a heuristic, make sure we are not reading too much data.
	if thres := int64(len(twain) / 100); rs.N > thres {
		t.Fatalf("read more data than expected: %d > %d", rs.N, thres)
	}
	rs.N = 0 // Reset the read count

	// Generate list of seek commands to try.
	type seekCommand struct {
		length int   // Number of bytes to read
		offset int64 // Seek to this offset
		whence int   // Whence value to use
	}
	vectors := []seekCommand{
		{length: 40, offset: int64(len(twain)) - 1, whence: io.SeekStart},
		{length: 40, offset: int64(len(twain)), whence: io.SeekStart},
		{length: 40, offset: int64(len(twain)) + 1, whence: io.SeekStart},
		{length: 40, offset: math.MaxInt64, whence: io.SeekStart},
		{length: 0, offset: 0, whence: io.SeekCurrent},
		{length: 13, offset: 15, whence: io.SeekStart},
		{length: 32, offset: 23, whence: io.SeekCurrent},
		{length: 32, offset: -23, whence: io.SeekCurrent},
		{length: 13, offset: -15, whence: io.SeekStart},
		{length: 100, offset: -15, whence: io.SeekEnd},
		{length: 0, offset: 0, whence: io.SeekCurrent},
		{length: 0, offset: 0, whence: io.SeekCurrent},
		{length: 32, offset: -34, whence: io.SeekCurrent},
		{length: 32, offset: -34, whence: io.SeekCurrent},
		{length: 2000, offset: 53, whence: io.SeekStart},
		{length: 2000, offset: int64(len(twain)) - 1000, whence: io.SeekStart},
		{length: 0, offset: 0, whence: io.SeekCurrent},
		{length: 100, offset: -int64(len(twain)), whence: io.SeekEnd},
		{length: 100, offset: -int64(len(twain)) - 1, whence: io.SeekEnd},
		{length: 0, offset: 0, whence: io.SeekStart},
		{length: 10, offset: 10, whence: io.SeekCurrent},
		{length: 10, offset: 10, whence: io.SeekCurrent},
		{length: 10, offset: 10, whence: io.SeekCurrent},
		{length: 10, offset: 10, whence: io.SeekCurrent},
		{length: 0, offset: 0, whence: -1},
	}

	// Add random values to seek list.
	for i := 0; i < 100; i++ {
		length, offset := rand.Intn(1<<11), rand.Int63n(int64(len(twain)))
		if length+int(offset) <= len(twain) {
			vectors = append(vectors, seekCommand{length, offset, io.SeekStart})
		}
	}

	// Read in reverse.
	vectors = append(vectors, seekCommand{0, 0, io.SeekEnd})
	for pos := int64(len(twain)); pos > 0; {
		n := int64(rand.Intn(1 << 11))
		if n > pos {
			n = pos
		}
		pos -= n
		vectors = append(vectors, seekCommand{int(n), pos, io.SeekStart})
	}

	// Execute all seek commands.
	var pos, totalLength int64
	for i, v := range vectors {
		// Emulate Seek logic.
		var wantPos int64
		switch v.whence {
		case io.SeekStart:
			wantPos = v.offset
		case io.SeekCurrent:
			wantPos = v.offset + pos
		case io.SeekEnd:
			wantPos = v.offset + int64(len(twain))
		default:
			wantPos = -1
		}

		// Perform actually (short-circuit if seek fails).
		wantFail := bool(wantPos < 0)
		gotPos, err := xr.Seek(v.offset, v.whence)
		if gotFail := bool(err != nil); gotFail != wantFail {
			if gotFail {
				t.Fatalf("test %d, unexpected failure: Seek(%d, %d) = (%d, %v)", i, v.offset, v.whence, pos, err)
			} else {
				t.Fatalf("test %d, unexpected success: Seek(%d, %d) = (%d, nil)", i, v.offset, v.whence, pos)
			}
		}
		if wantFail {
			continue
		}
		if gotPos != wantPos {
			t.Fatalf("test %d, offset mismatch: got %d, want %d", i, gotPos, wantPos)
		}

		// Read and verify some length of bytes.
		var want []byte
		if wantPos < int64(len(twain)) {
			want = twain[wantPos:]
		}
		if len(want) > v.length {
			want = want[:v.length]
		}
		got, err := ioutil.ReadAll(io.LimitReader(xr, int64(v.length)))
		if err != nil {
			t.Fatalf("test %v, unexpected error: ReadAll() = %v", i, err)
		}
		if got, want, ok := testutil.BytesCompare(got, want); !ok {
			t.Fatalf("test %v, mismatching output:\ngot  %s\nwant %s", i, got, want)
		}

		pos = gotPos + int64(len(got))
		totalLength += int64(v.length)
	}

	// As a heuristic, make sure we are not reading too much data.
	if thres := 2 * totalLength; rs.N > thres {
		t.Fatalf("read more data than expected: %d > %d", rs.N, thres)
	}
}

func TestRecursiveReader(t *testing.T) {
	twain := testutil.MustLoadFile("../testdata/twain.txt")

	const numIters = 5
	var bb bytes.Buffer

	// Recursively compress the same input data multiple times using XFLATE.
	// Run as a closured function to ensure defer statements execute.
	func() {
		wlast := io.Writer(&bb) // Latest writer
		for i := 0; i < numIters; i++ {
			xw, err := NewWriter(wlast, &WriterConfig{ChunkSize: 1 << uint(10+i)})
			if err != nil {
				t.Fatalf("unexpected error: NewWriter() = %v", err)
			}
			defer func() {
				if err := xw.Close(); err != nil {
					t.Fatalf("unexpected error: Close() = %v", err)
				}
			}()
			wlast = xw
		}
		if _, err := wlast.Write(twain); err != nil {
			t.Fatalf("unexpected error: Write() = %v", err)
		}
	}()

	// Recursively decompress the same input stream multiple times.
	func() {
		rlast := io.ReadSeeker(bytes.NewReader(bb.Bytes()))
		for i := 0; i < numIters; i++ {
			xr, err := NewReader(rlast, nil)
			if err != nil {
				t.Fatalf("unexpected error: NewReader() = %v", err)
			}
			defer func() {
				if err := xr.Close(); err != nil {
					t.Fatalf("unexpected error: Close() = %v", err)
				}
			}()
			rlast = xr
		}

		buf := make([]byte, 321)
		if _, err := rlast.Seek(int64(len(twain))/2, io.SeekStart); err != nil {
			t.Fatalf("unexpected error: Seek() = %v", err)
		}
		if _, err := io.ReadFull(rlast, buf); err != nil {
			t.Fatalf("unexpected error: Read() = %v", err)
		}
		if got, want := string(buf), string(twain[len(twain)/2:][:321]); got != want {
			t.Errorf("output mismatch:\ngot  %q\nwant %q", got, want)
		}
	}()
}

// BenchmarkReader benchmarks the overhead of the XFLATE format over DEFLATE.
// Thus, it intentionally uses a very small chunk size with no compression.
// This benchmark reads the input file in reverse to excite poor behavior.
func BenchmarkReader(b *testing.B) {
	rand := rand.New(rand.NewSource(0))
	twain := testutil.MustLoadFile("../testdata/twain.txt")
	bb := bytes.NewBuffer(make([]byte, 0, 2*len(twain)))
	xr := new(Reader)
	lr := new(io.LimitedReader)

	xw, _ := NewWriter(bb, &WriterConfig{Level: NoCompression, ChunkSize: 1 << 10})
	xw.Write(twain)
	xw.Close()

	b.ReportAllocs()
	b.SetBytes(int64(len(twain)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rand.Seed(0)
		rd := bytes.NewReader(bb.Bytes())
		if err := xr.Reset(rd); err != nil {
			b.Fatalf("unexpected error: Reset() = %v", err)
		}

		// Read sections of the input in reverse.
		for pos := int64(len(twain)); pos > 0; {
			// Random section size.
			n := int64(rand.Intn(1 << 11))
			if n > pos {
				n = pos
			}
			pos -= n

			// Read the given section.
			if _, err := xr.Seek(pos, io.SeekStart); err != nil {
				b.Fatalf("unexpected error: Seek() = %v", err)
			}
			*lr = io.LimitedReader{R: xr, N: n}
			if _, err := io.Copy(ioutil.Discard, lr); err != nil {
				b.Fatalf("unexpected error: Copy() = %v", err)
			}
		}
	}
}
