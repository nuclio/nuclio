// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build ignore

// xflate_stats is used to investigate trade-offs in XFLATE design.
//
// The XFLATE format extends the DEFLATE format in order to allow for limited
// random access reading. This is achieved by individually compressing the
// input data in chunks (so that each chunk can be decompressed by itself),
// and by storing an index recording the size of every chunk (so that each
// chunk can be located).
//
// However, this adds overhead and diminishes the compression ratio slightly.
// In order to investigate the cost of chunking, this program computes the total
// compressed size when the input is compressed as a single stream or when
// compressed in chunks of some fixed size.
//
// Chunking diminishes compression ratio because the LZ77 dictionary is reset
// for each chunk, reducing the amount of savings that could have been gained
// by a backwards match to previous data. Secondly, the XFLATE format requires
// that each chunk terminates with a SYNC_FLUSH marker of ~5 bytes.
//
// In addition to the costs of chunking itself, there is also the cost of
// storing the index. The index is a list of (rawSize, compSize) tuples where
// each record contains information about the uncompressed input size (rawSize)
// and compressed output size (compSize) of each chunk.
//
// In order to reduce the index size, multiple techniques were explored:
//	* Compressing the index itself using DEFLATE
//	* Row-oriented vs. column-oriented layout of records; that is, row-oriented
//	layout has each (rawSize, compSize) tuple laid out one after the other,
//	while column-oriented has all rawSizes laid out as an array followed by all
//	compSizes laid out as an array
//	* Fixed-length vs. variable-length integers; that is, to store size values
//	as uint64s or to use some variable-length encoding
//	* Regular encoding vs. Delta encoding; that is, delta encoding encodes each
//	value as the difference between that value and the previous value
//
// However, some of these techniques do not make sense unless done in
// conjunction with another technique. For example:
//	* Row-oriented vs. column-oriented is useless without compression
//	* Delta encoding does not make sense without variable-length integers
//
// For this reason, the following table shows what techniques were used for the
// given format names listed below:
//
//	         +-------------- Raw: The index is uncompressed
//	         |   +---------- Row: The index is row-oriented
//	         |   |   +------ Fix: Index records use fixed-length uint64s
//	         |   |   |   +-- Reg: Index records are not delta encoded
//	         |   |   |   |
//	        Raw Row Fix Reg
//	RawFix  [X] [?] [X] [X]
//	RawVar  [X] [?] [ ] [X]
//	RawDlt  [X] [?] [ ] [ ]
//	RowFix  [ ] [X] [X] [X]
//	RowVar  [ ] [X] [ ] [X]
//	RowDlt  [ ] [X] [ ] [ ]
//	ColFix  [ ] [ ] [X] [X]
//	ColVar  [ ] [ ] [ ] [X]
//	ColDlt  [ ] [ ] [ ] [ ]
//
// As seen, the Raw* formats do not care about the orientation of the index
// since it is not compressed. Also, all *Dlt formats uses delta encoding
// in conjunction with variable-length encoding.
//
// In summary, the greatest factor for overhead cost is the chunk size.
// Smaller chunks reduce efficiency due to more frequent LZ77 dictionary resets
// and also increases the total number of chunks needed, thus also increasing
// the index size. On the other hand, larger chunks also diminishes the
// effectiveness of random access reading since more data must be discarded to
// start reading from some given offset. Thus, the choice of chunk size is not
// hard-coded by the XFLATE format and this tool can help identify the
// appropriate chunk size.
package main

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/dsnet/compress/xflate/internal/meta"
)

func init() { log.SetFlags(log.Lshortfile) }

func main() {
	inputFile := flag.String("f", "-", "path to input file")
	chunkSize := flag.Int("n", 64*1024, "compress the input in n-sized chunks")
	compLevel := flag.Int("l", flate.DefaultCompression, "compression level")
	metaEncode := flag.Bool("m", false, "encode index using XFLATE meta encoding")
	flag.Parse()

	// Open the input file.
	var r io.Reader
	if *inputFile != "-" {
		f, err := os.Open(*inputFile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		r = f
	} else {
		r = os.Stdin
	}

	// Compute the streamed and chunked record values.
	var chnkRec indexRecord
	strmRec, chnkRecs := computeRecords(r, *compLevel, *chunkSize)
	for _, r := range chnkRecs {
		chnkRec.rawSize += r.rawSize
		chnkRec.compSize += r.compSize
	}

	var n int
	var pr indexRecord // Previous record
	var brf, brv, brd, bcf, bcv, bcd bytes.Buffer
	buf := make([]byte, binary.MaxVarintLen64)

	// Row-based index format.
	pr = indexRecord{}
	for _, r := range chnkRecs {
		binary.LittleEndian.PutUint64(buf, uint64(r.rawSize))
		brf.Write(buf[:8])
		binary.LittleEndian.PutUint64(buf, uint64(r.compSize))
		brf.Write(buf[:8])

		n = binary.PutUvarint(buf, uint64(r.rawSize))
		brv.Write(buf[:n])
		n = binary.PutUvarint(buf, uint64(r.compSize))
		brv.Write(buf[:n])

		n = binary.PutVarint(buf, r.rawSize-pr.rawSize)
		brd.Write(buf[:n])
		n = binary.PutVarint(buf, r.compSize-pr.compSize)
		brd.Write(buf[:n])

		pr = r
	}

	// Column-based index format.
	pr = indexRecord{}
	for _, r := range chnkRecs {
		binary.LittleEndian.PutUint64(buf, uint64(r.rawSize))
		bcf.Write(buf[:8])

		n = binary.PutUvarint(buf, uint64(r.rawSize))
		bcv.Write(buf[:n])

		n = binary.PutVarint(buf, r.rawSize-pr.rawSize)
		bcd.Write(buf[:n])

		pr.rawSize = r.rawSize
	}
	for _, r := range chnkRecs {
		binary.LittleEndian.PutUint64(buf, uint64(r.compSize))
		bcf.Write(buf[:8])

		n = binary.PutUvarint(buf, uint64(r.compSize))
		bcv.Write(buf[:n])

		n = binary.PutVarint(buf, r.compSize-pr.compSize)
		bcd.Write(buf[:n])

		pr.compSize = r.compSize
	}

	pf := func(a, b int64) float64 { return 100.0 * (float64(a) / float64(b)) }
	me := func(b []byte) []byte { return b }
	if *metaEncode {
		me = encode
	}

	// Print basic statistics about the input and output.
	ns, nc := strmRec.compSize, chnkRec.compSize
	ps, pc := pf(ns-ns, ns), pf(nc-ns, ns)
	fmt.Printf("rawSize:          %d\n", strmRec.rawSize)   // Uncompressed input size
	fmt.Printf("strmRec.compSize: %d (%+0.2f%%)\n", ns, ps) // Total compressed size as a single stream
	fmt.Printf("chnkRec.compSize: %d (%+0.2f%%)\n", nc, pc) // Total compressed size as individual chunks
	fmt.Printf("chunkSize:        %d\n", *chunkSize)        // Individual chunk size
	fmt.Printf("numChunks:        %d\n", len(chnkRecs))     // Total number of chunks
	fmt.Println()

	// Uncompressed index formats.
	nf := int64(len(me(brf.Bytes())))
	nv := int64(len(me(brv.Bytes())))
	nd := int64(len(me(brd.Bytes())))
	fmt.Printf("RawFix: %d (%+0.2f%%)\n", nf, pf(nf, ns)) // Fixed-length uint64s
	fmt.Printf("RawVar: %d (%+0.2f%%)\n", nv, pf(nv, ns)) // Variable-length integers (VLI)
	fmt.Printf("RawDlt: %d (%+0.2f%%)\n", nd, pf(nd, ns)) // VLI with delta encoding
	fmt.Println()

	// Compressed row-oriented index formats.
	nrf := int64(len(me(compress(brf.Bytes(), *compLevel))))
	nrv := int64(len(me(compress(brv.Bytes(), *compLevel))))
	nrd := int64(len(me(compress(brd.Bytes(), *compLevel))))
	fmt.Printf("RowFix: %d (%+0.2f%%)\n", nrf, pf(nrf, ns)) // Fixed-length uint64s
	fmt.Printf("RowVar: %d (%+0.2f%%)\n", nrv, pf(nrv, ns)) // Variable-length integers (VLI)
	fmt.Printf("RowDlt: %d (%+0.2f%%)\n", nrd, pf(nrd, ns)) // VLI with delta encoding
	fmt.Println()

	// Compressed column-oriented index formats.
	ncf := int64(len(me(compress(bcf.Bytes(), *compLevel))))
	ncv := int64(len(me(compress(bcv.Bytes(), *compLevel))))
	ncd := int64(len(me(compress(bcd.Bytes(), *compLevel))))
	fmt.Printf("ColFix: %d (%+0.2f%%)\n", ncf, pf(ncf, ns)) // Fixed-length uint64s
	fmt.Printf("ColVar: %d (%+0.2f%%)\n", ncv, pf(ncv, ns)) // Variable-length integers (VLI)
	fmt.Printf("ColDlt: %d (%+0.2f%%)\n", ncd, pf(ncd, ns)) // VLI with delta encoding
	fmt.Println()
}

// countWriter counts and discards all bytes written to it.
type countWriter int64

func (cw *countWriter) Write(b []byte) (int, error) {
	*(*int64)(cw) += int64(len(b))
	return len(b), nil
}

type indexRecord struct {
	rawSize  int64 // Size when decompressed
	compSize int64 // Size when compressed
}

// computeRecords computes the records the raw input size and the compressed
// output size. strmRec is a single record for when the input is compressed as
// a single stream. chnkRecs is a list of records for when the input is
// compressed individually as chunks.
func computeRecords(r io.Reader, lvl, chnkSize int) (strmRec indexRecord, chnkRecs []indexRecord) {
	var cw1, cw2 countWriter
	zw1, _ := flate.NewWriter(&cw1, lvl) // Streamed compressor
	zw2, _ := flate.NewWriter(&cw2, lvl) // Chunked compressor
	buf := make([]byte, chnkSize)
	for {
		// Read a full chunks worth of data.
		cnt, err := io.ReadFull(r, buf)
		strmRec.rawSize += int64(cnt)
		if err == io.EOF {
			break
		}

		// Write chunk to both compressors.
		if _, err := zw1.Write(buf[:cnt]); err != nil {
			log.Fatal(err)
		}
		if _, err := zw2.Write(buf[:cnt]); err != nil {
			log.Fatal(err)
		}

		// Flush the chunked compressor, append the record, and reset.
		if err := zw2.Flush(); err != nil {
			log.Fatal(err)
		}
		chnkRecs = append(chnkRecs, indexRecord{rawSize: int64(cnt), compSize: int64(cw2)})
		cw2 = 0
		zw2.Reset(&cw2)

		if err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
	}

	// Flush the streamed compressor and record the compressed size.
	if err := zw1.Flush(); err != nil {
		log.Fatal(err)
	}
	strmRec.compSize = int64(cw1)
	return strmRec, chnkRecs
}

// compress compresses the input buffer at the given level.
func compress(b []byte, lvl int) []byte {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, lvl)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := io.Copy(w, bytes.NewReader(b)); err != nil {
		log.Fatal(err)
	}
	if err := w.Close(); err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
}

// encode encodes the input using XFLATE's meta encoding.
func encode(b []byte) []byte {
	var buf bytes.Buffer
	mw := meta.NewWriter(&buf)
	mw.FinalMode = meta.FinalMeta
	if _, err := io.Copy(mw, bytes.NewReader(b)); err != nil {
		log.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
}
