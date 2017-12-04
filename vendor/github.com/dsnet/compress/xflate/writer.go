// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package xflate

import (
	"encoding/binary"
	"hash/crc32"
	"io"

	"github.com/dsnet/compress/internal/errors"
	"github.com/dsnet/compress/xflate/internal/meta"
)

// A Writer is an io.Writer that can write the XFLATE format.
// The XFLATE stream outputted by this Writer can be read by both Reader and
// flate.Reader.
type Writer struct {
	// These statistics fields are automatically updated by Writer.
	// It is safe to set these values to any arbitrary value.
	InputOffset  int64 // Total number of bytes issued to Write
	OutputOffset int64 // Total number of bytes written to underlying io.Writer

	wr io.Writer
	mw meta.Writer  // Meta encoder used to write the index and footer
	zw *flateWriter // DEFLATE compressor

	idx  index // Index table of seekable offsets
	nidx int64 // Number of records per index
	nchk int64 // Raw size of each independent chunk
	err  error // Persistent error

	// The following fields are embedded here to reduce memory allocations.
	scratch [64]byte
}

// WriterConfig configures the Writer.
// The zero value for any field uses the default value for that field type.
type WriterConfig struct {
	// Underlying DEFLATE compression level.
	//
	// This compression level will be passed directly to the underlying DEFLATE
	// compressor. Higher values provide better compression ratio at the expense
	// of CPU time.
	Level int

	// Uncompressed size of each independent chunk.
	//
	// Each chunk will be compressed independently. This has that advantage that
	// the chunk can be decompressed without knowledge about the preceding
	// chunks, but has the disadvantage that it reduces the compression ratio.
	// Smaller ChunkSizes provide better random access properties, while larger
	// sizes provide better compression ratio.
	ChunkSize int64

	// The number of records in each index.
	//
	// When this number is reached, the index is automatically flushed. This is
	// done to ensure that there is some limit on the amount of memory needed to
	// represent the index. A negative value indicates that the Writer will
	// not automatically flush the index.
	//
	// The multiplication of the IndexSize and the size of each record (24 B)
	// gives an approximation for how much memory the index will occupy.
	// The multiplication of the IndexSize and the ChunkSize gives an
	// approximation for how much uncompressed data each index represents.
	IndexSize int64

	_ struct{} // Blank field to prevent unkeyed struct literals
}

// NewWriter creates a new Writer writing to the given writer.
// It is the caller's responsibility to call Close to complete the stream.
//
// If conf is nil, then default configuration values are used. Writer copies
// all configuration values as necessary and does not store conf.
func NewWriter(wr io.Writer, conf *WriterConfig) (*Writer, error) {
	var lvl int
	var nchk, nidx int64
	if conf != nil {
		lvl = conf.Level
		switch {
		case conf.ChunkSize < 0:
			return nil, errorf(errors.Invalid, "invalid chunk size: %d", conf.ChunkSize)
		case conf.ChunkSize > 0:
			nchk = conf.ChunkSize
		}
		switch {
		case conf.IndexSize < 0:
			nidx = -1
		case conf.IndexSize > 0:
			nidx = conf.IndexSize
		}
	}

	zw, err := newFlateWriter(wr, lvl)
	if err != nil {
		return nil, err
	}
	xw := &Writer{wr: wr, zw: zw, nchk: nchk, nidx: nidx}
	xw.Reset(wr)
	return xw, nil
}

// Reset discards the Writer's state and makes it equivalent to the result
// of a call to NewWriter, but writes to wr instead. Any configurations from
// a prior call to NewWriter will be preserved.
//
// This is used to reduce memory allocations.
func (xw *Writer) Reset(wr io.Writer) error {
	*xw = Writer{
		wr:   wr,
		mw:   xw.mw,
		zw:   xw.zw,
		nchk: xw.nchk,
		nidx: xw.nidx,
		idx:  xw.idx,
	}
	if xw.zw == nil {
		xw.zw, _ = newFlateWriter(wr, DefaultCompression)
	} else {
		xw.zw.Reset(wr)
	}
	if xw.nchk == 0 {
		xw.nchk = DefaultChunkSize
	}
	if xw.nidx == 0 {
		xw.nidx = DefaultIndexSize
	}
	xw.idx.Reset()
	return nil
}

// Write writes the compressed form of buf to the underlying io.Writer.
// This automatically breaks the input into multiple chunks, writes them out,
// and records the sizes of each chunk in the index table.
func (xw *Writer) Write(buf []byte) (int, error) {
	if xw.err != nil {
		return 0, xw.err
	}

	var n, cnt int
	for len(buf) > 0 && xw.err == nil {
		// Flush chunk if necessary.
		remain := xw.nchk - xw.zw.InputOffset
		if remain <= 0 {
			xw.err = xw.Flush(FlushFull)
			continue
		}
		if remain > int64(len(buf)) {
			remain = int64(len(buf))
		}

		// Compress data for current chunk.
		offset := xw.zw.OutputOffset
		n, xw.err = xw.zw.Write(buf[:remain])
		xw.OutputOffset += xw.zw.OutputOffset - offset
		buf = buf[n:]
		cnt += n
	}

	xw.InputOffset += int64(cnt)
	return cnt, xw.err
}

// Flush flushes the current write buffer to the underlying writer.
// Flushing is entirely optional and should be used sparingly.
func (xw *Writer) Flush(mode FlushMode) error {
	if xw.err != nil {
		return xw.err
	}

	switch mode {
	case FlushSync:
		offset := xw.zw.OutputOffset
		xw.err = xw.zw.Flush()
		xw.OutputOffset += xw.zw.OutputOffset - offset
		return xw.err
	case FlushFull:
		if xw.err = xw.Flush(FlushSync); xw.err != nil {
			return xw.err
		}
		xw.idx.AppendRecord(xw.zw.OutputOffset, xw.zw.InputOffset, deflateType)
		xw.zw.Reset(xw.wr)
		if int64(len(xw.idx.Records)) == xw.nidx {
			xw.err = xw.Flush(FlushIndex)
		}
		return xw.err
	case FlushIndex:
		if xw.zw.InputOffset+xw.zw.OutputOffset > 0 {
			if err := xw.Flush(FlushFull); err != nil {
				return err
			}
		}
		xw.err = xw.encodeIndex(&xw.idx)
		backSize := xw.idx.IndexSize
		xw.idx.Reset()
		xw.idx.BackSize = backSize
		return xw.err
	default:
		return errorf(errors.Invalid, "invalid flush mode: %d", mode)
	}
}

// Close ends the XFLATE stream and flushes all buffered data.
// This method automatically writes an index if any chunks have been written
// since the last FlushIndex.
func (xw *Writer) Close() error {
	if xw.err == errClosed {
		return nil
	}
	if xw.err != nil {
		return xw.err
	}

	// Flush final index.
	if xw.zw.OutputOffset+xw.zw.InputOffset > 0 || len(xw.idx.Records) > 0 {
		xw.err = xw.Flush(FlushIndex)
		if xw.err != nil {
			return xw.err
		}
	}

	// Encode the footer.
	err := xw.encodeFooter(xw.idx.BackSize)
	if err != nil {
		xw.err = err
	} else {
		xw.err = errClosed
	}
	return err
}

// encodeIndex encodes the index into a meta encoded stream.
// The index.Records and index.BackSize fields must be populated.
// The index.IndexSize field will be populated upon successful write.
func (xw *Writer) encodeIndex(index *index) error {
	// Helper function to write VLIs.
	var crc uint32
	var errVLI error
	writeVLI := func(x int64) {
		b := xw.scratch[:binary.PutUvarint(xw.scratch[:], uint64(x))]
		crc = crc32.Update(crc, crc32.MakeTable(crc32.IEEE), b)
		if _, err := xw.mw.Write(b); err != nil {
			errVLI = errWrap(err)
		}
	}

	// Write the index.
	xw.mw.Reset(xw.wr)
	defer func() { xw.OutputOffset += xw.mw.OutputOffset }()
	xw.mw.FinalMode = meta.FinalMeta
	writeVLI(index.BackSize)
	writeVLI(int64(len(index.Records)))
	writeVLI(index.LastRecord().CompOffset)
	writeVLI(index.LastRecord().RawOffset)
	var preRec record
	for _, rec := range index.Records {
		writeVLI(rec.CompOffset - preRec.CompOffset)
		writeVLI(rec.RawOffset - preRec.RawOffset)
		preRec = rec
	}
	if errVLI != nil {
		return errWrap(errVLI)
	}

	binary.LittleEndian.PutUint32(xw.scratch[:], crc)
	if _, err := xw.mw.Write(xw.scratch[:4]); err != nil {
		return errWrap(err)
	}
	if err := xw.mw.Close(); err != nil {
		return errWrap(err)
	}
	index.IndexSize = xw.mw.OutputOffset // Record the encoded size
	return nil
}

// encodeFooter writes the final footer, encoding the provided backSize into it.
func (xw *Writer) encodeFooter(backSize int64) error {
	var n int
	n += copy(xw.scratch[n:], magic[:])
	n += binary.PutUvarint(xw.scratch[n:], uint64(backSize))

	xw.mw.Reset(xw.wr)
	defer func() { xw.OutputOffset += xw.mw.OutputOffset }()
	xw.mw.FinalMode = meta.FinalStream
	if _, err := xw.mw.Write(xw.scratch[:n]); err != nil {
		return errWrap(err)
	}
	if err := xw.mw.Close(); err != nil {
		return errWrap(err)
	}
	if xw.mw.NumBlocks != 1 {
		return errorf(errors.Internal, "footer was not a single block")
	}
	return nil
}
