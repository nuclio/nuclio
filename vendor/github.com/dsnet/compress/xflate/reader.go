// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package xflate

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"
	"io/ioutil"
	"math"

	"github.com/dsnet/compress/internal/errors"
	"github.com/dsnet/compress/xflate/internal/meta"
)

// chunk is a tuple of raw (uncompressed) size and compressed size for a chunk.
type chunk struct {
	csize, rsize int64
	typ          int
}

// chunkReader wraps an io.Reader by appending an endBlock marker at the end.
//
// Attempting to decompress each chunk ad-verbatim with a DEFLATE decompressor
// will lead to an io.ErrUnexpectedEOF error since the chunk does not terminate
// with a DEFLATE block with the final bit set. The chunkReader appends the
// endBlock to the stream so that the decompressor knows when to terminate.
// It also tracks the last 4 bytes of the chunk for sync marker verification.
type chunkReader struct {
	rd   io.LimitedReader
	sync uint32 // Last 4 bytes from underlying stream
	end  []byte // Bytes of the endBlock marker
}

func (cr *chunkReader) Reset(rd io.Reader, n int64) {
	*cr = chunkReader{rd: io.LimitedReader{R: rd, N: n}}
}

func (cr *chunkReader) Read(buf []byte) (int, error) {
	max := func(a, b int) int {
		if a < b {
			return b
		}
		return a
	}

	// Append endBlock marker.
	if cr.end != nil {
		n := copy(buf, cr.end)
		cr.end = cr.end[n:]
		if len(cr.end) == 0 {
			return n, io.EOF
		}
		return n, nil
	}

	// Read from underlying data stream.
	n, err := cr.rd.Read(buf)
	for i := max(n-4, 0); i < n; i++ {
		cr.sync = (cr.sync << 8) | uint32(buf[i])
	}
	if err == io.EOF {
		cr.end = endBlock[:]
		err = nil
	}
	return n, err
}

// A Reader is an io.ReadSeeker that can read the XFLATE format. Only the stream
// produced by Writer (or some other valid XFLATE stream) can be read by Reader.
// Regular DEFLATE streams produced by flate.Writer cannot be read by Reader.
type Reader struct {
	// TODO(dsnet): Export index information somehow.

	rd io.ReadSeeker
	mr meta.Reader  // Meta decoder used to read the index and footer
	cr chunkReader  // Wraps rd before being passed into zr
	zr *flateReader // DEFLATE decompressor

	ri      int   // Current record number
	offset  int64 // Current raw offset
	discard int64 // Number of bytes to discard to reach offset
	idx     index // Index table of seekable offsets
	chk     chunk // Information about the current chunk
	err     error // Persistent error

	// The following fields are embedded here to reduce memory allocations.
	lr     io.LimitedReader
	br, bw bytes.Buffer
	idxs   []index
	chunks []chunk
}

// ReaderConfig configures the Reader.
// There are currently no configuration options for Reader.
type ReaderConfig struct {
	_ struct{} // Blank field to prevent unkeyed struct literals
}

// NewReader creates a new Reader reading the given reader rs. This reader can
// only decompress files in the XFLATE format. If the underlying stream is
// regular DEFLATE and not XFLATE, then this returns error.
//
// Regardless of the current offset in rs, this function Seeks to the end of rs
// in order to determine the total compressed size. The Reader returned has its
// offset set to the start of the stream.
//
// If conf is nil, then default configuration values are used. Reader copies
// all configuration values as necessary and does not store conf.
func NewReader(rs io.ReadSeeker, conf *ReaderConfig) (*Reader, error) {
	xr := new(Reader)
	err := xr.Reset(rs)
	return xr, err
}

// Reset discards the Reader's state and makes it equivalent to the result
// of a call to NewReader, but reading from rd instead. This method may return
// an error if it is unable to parse the index.
//
// This is used to reduce memory allocations.
func (xr *Reader) Reset(rs io.ReadSeeker) error {
	*xr = Reader{
		rd:  rs,
		mr:  xr.mr,
		zr:  xr.zr,
		idx: xr.idx,

		br:     xr.br,
		bw:     xr.bw,
		idxs:   xr.idxs,
		chunks: xr.chunks,
	}
	if xr.zr == nil {
		xr.zr, _ = newFlateReader(nil)
	}
	xr.idx.Reset()

	// Read entire index.
	var backSize, footSize int64
	if backSize, footSize, xr.err = xr.decodeFooter(); xr.err != nil {
		return xr.err
	}
	if xr.err = xr.decodeIndexes(backSize); xr.err != nil {
		return xr.err
	}
	if !xr.idx.AppendRecord(footSize, 0, footerType) {
		xr.err = errCorrupted
		return xr.err
	}

	// Setup initial chunk reader.
	_, xr.err = xr.Seek(0, io.SeekStart)
	return xr.err
}

// Read reads decompressed data from the underlying io.Reader.
// This method automatically proceeds to the next chunk when the current one
// has been fully read.
func (xr *Reader) Read(buf []byte) (int, error) {
	if xr.err != nil {
		return 0, xr.err
	}

	// Discard some data to reach the expected raw offset.
	if xr.discard > 0 {
		var n int64
		xr.lr = io.LimitedReader{R: xr.zr, N: xr.discard}
		n, xr.err = io.Copy(ioutil.Discard, &xr.lr)
		if xr.err != nil {
			return 0, xr.err
		}
		if n != xr.discard {
			xr.err = errCorrupted
			return 0, xr.err
		}
		xr.discard = 0
	}

	var cnt int
	for cnt == 0 && xr.err == nil {
		cnt, xr.err = xr.zr.Read(buf)
		xr.offset += int64(cnt)
		if xr.err == io.EOF {
			xr.err = nil // Clear io.EOF temporarily

			// Verify that the compressed section ends with an empty raw block.
			if xr.chk.typ == deflateType && xr.cr.sync != 0x0000ffff {
				xr.err = errCorrupted
				break
			}

			// Verify that the compressed and raw sizes match.
			if xr.chk.typ != footerType {
				xr.chk.csize += int64(len(endBlock)) // Side of effect of using chunkReader
			}
			if xr.chk.csize != xr.zr.InputOffset || xr.chk.rsize != xr.zr.OutputOffset {
				xr.err = errCorrupted
				break
			}

			// Seek to next chunk.
			if _, xr.err = xr.Seek(xr.offset, io.SeekStart); xr.err != nil {
				break
			}
			if xr.chk.typ == unknownType {
				xr.err = io.EOF
			}
		}
	}

	return cnt, xr.err
}

// Seek sets the offset for the next Read operation, interpreted according to
// the whence value provided. It is permitted to seek to offsets in the middle
// of a compressed chunk. The next call to Read will automatically discard some
// number of bytes before returning the requested data.
func (xr *Reader) Seek(offset int64, whence int) (int64, error) {
	if xr.err != nil && xr.err != io.EOF {
		return 0, xr.err
	}

	// Determine which position to seek to.
	var pos int64
	end := xr.idx.LastRecord().RawOffset
	switch whence {
	case io.SeekStart:
		pos = offset
	case io.SeekCurrent:
		pos = xr.offset + offset
	case io.SeekEnd:
		pos = end + offset
	default:
		return 0, errorf(errors.Invalid, "invalid whence: %d", whence)
	}
	if pos < 0 {
		return 0, errorf(errors.Invalid, "negative position: %d", pos)
	}

	// As an optimization if the new position is within the current chunk,
	// then just adjust the discard value.
	discard := pos - xr.offset
	remain := xr.chk.rsize - xr.zr.OutputOffset
	if discard > 0 && remain > 0 && discard < remain {
		xr.offset, xr.discard = pos, discard
		return pos, nil
	}

	// Query the index for the chunk to start decoding from.
	// Attempt to use the subsequent record before resorting to binary search.
	prev, curr := xr.idx.GetRecords(xr.ri)
	if !(prev.RawOffset <= pos && pos <= curr.RawOffset) {
		xr.ri = xr.idx.Search(pos)
		prev, curr = xr.idx.GetRecords(xr.ri)
	}
	xr.ri++
	if xr.ri > len(xr.idx.Records) {
		xr.ri = len(xr.idx.Records)
	}

	// Setup a chunk reader at the given position.
	xr.chk = chunk{
		csize: curr.CompOffset - prev.CompOffset,
		rsize: curr.RawOffset - prev.RawOffset,
		typ:   curr.Type,
	}
	xr.offset, xr.discard = pos, pos-prev.RawOffset
	if pos > end {
		// In case pos is really large, only discard data that actually exists.
		xr.discard = end - prev.RawOffset
	}
	_, xr.err = xr.rd.Seek(prev.CompOffset, io.SeekStart)
	xr.cr.Reset(xr.rd, xr.chk.csize)
	xr.zr.Reset(&xr.cr)
	return pos, xr.err
}

// Close ends the XFLATE stream.
func (xr *Reader) Close() error {
	if xr.err == errClosed {
		return nil
	}
	if xr.err != nil && xr.err != io.EOF {
		return xr.err
	}
	xr.err = errClosed
	return nil
}

// decodeIndexes iteratively decodes all of the indexes in the XFLATE stream.
// Even if the index is fragmented in the source stream, this method will merge
// all of the index fragments into a single index table.
func (xr *Reader) decodeIndexes(backSize int64) error {
	pos, err := xr.rd.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	// Read all indexes.
	var compSize int64
	xr.idxs = xr.idxs[:0]
	for {
		// Seek backwards past index and compressed blocks.
		newPos := pos - (backSize + compSize)
		if newPos < 0 || newPos > pos {
			return errCorrupted // Integer overflow for new seek position
		}
		if pos, err = xr.rd.Seek(newPos, io.SeekStart); err != nil {
			return err
		}
		if backSize == 0 {
			break
		}

		// Read the index.
		if cap(xr.idxs) > len(xr.idxs) {
			xr.idxs = xr.idxs[:len(xr.idxs)+1]
		} else {
			xr.idxs = append(xr.idxs, index{})
		}
		idx := &xr.idxs[len(xr.idxs)-1]
		idx.Reset()
		idx.IndexSize = backSize
		if err = xr.decodeIndex(idx); err != nil {
			return err
		}
		backSize, compSize = idx.BackSize, idx.LastRecord().CompOffset
	}
	if pos != 0 {
		return errCorrupted
	}

	// Compact all indexes into one.
	for i := len(xr.idxs) - 1; i >= 0; i-- {
		idx := xr.idxs[i]
		if !xr.idx.AppendIndex(&idx) {
			return errCorrupted
		}
		if !xr.idx.AppendRecord(idx.IndexSize, 0, indexType) {
			return errCorrupted
		}
	}
	return nil
}

// decodeIndex decodes the index from a meta encoded stream.
// The current offset must be set to the start of the encoded index and
// index.IndexSize must be populated. If successful, the index.Records and
// index.BackSize fields will be populated. This method will attempt to reset
// the read offset to the start of the index.
func (xr *Reader) decodeIndex(idx *index) error {
	// Helper function to read VLIs.
	var errVLI error
	readVLI := func() int64 {
		x, n := binary.Uvarint(xr.bw.Bytes())
		if n <= 0 || x > math.MaxInt64 {
			errVLI = errCorrupted
			return 0
		}
		xr.bw.Next(n)
		return int64(x)
	}

	// Read the index and restore the underlying reader offset.
	xr.br.Reset()
	xr.lr = io.LimitedReader{R: xr.rd, N: idx.IndexSize}
	n, err := io.Copy(&xr.br, &xr.lr)
	if err != nil {
		return err
	}
	if _, err := xr.rd.Seek(-n, io.SeekCurrent); err != nil {
		return err
	}

	// Parse the index.
	var crc uint32
	xr.chunks = xr.chunks[:0]
	xr.bw.Reset()
	xr.mr.Reset(&xr.br)
	if _, err := io.Copy(&xr.bw, &xr.mr); err != nil {
		return errWrap(err)
	}
	if xr.bw.Len() > 4 {
		crc = crc32.ChecksumIEEE(xr.bw.Bytes()[:xr.bw.Len()-4])
	}
	idx.BackSize = readVLI()
	numRecs := readVLI()
	totalCompSize := readVLI()
	totalRawSize := readVLI()
	if errVLI != nil {
		return errVLI
	}

	for i := int64(0); i < numRecs; i++ {
		xr.chunks = append(xr.chunks, chunk{readVLI(), readVLI(), 0})
	}
	if xr.bw.Len() != 4 || binary.LittleEndian.Uint32(xr.bw.Bytes()) != crc {
		return errCorrupted
	}
	if xr.mr.FinalMode != meta.FinalMeta {
		return errCorrupted
	}
	if xr.mr.InputOffset != idx.IndexSize {
		return errCorrupted
	}

	// Convert individual index sizes to be absolute offsets.
	for _, chk := range xr.chunks {
		if chk.csize <= 4 {
			return errCorrupted // Every chunk has a sync marker
		}
		if !idx.AppendRecord(chk.csize, chk.rsize, deflateType) {
			return errCorrupted
		}
	}
	lastRec := idx.LastRecord()
	if lastRec.CompOffset != totalCompSize || lastRec.RawOffset != totalRawSize {
		return errCorrupted
	}
	return nil
}

// decodeFooter seeks to the end of the stream, searches for the footer
// and decodes it. If successful, it will return the backSize for the preceding
// index and the size of the footer itself. This method will attempt to reset
// the read offset to the start of the footer.
func (xr *Reader) decodeFooter() (backSize, footSize int64, err error) {
	// Read the last few bytes of the stream.
	end, err := xr.rd.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, 0, err
	}
	if end > meta.MaxEncBytes {
		end = meta.MaxEncBytes
	}
	if _, err := xr.rd.Seek(-end, io.SeekEnd); err != nil {
		return 0, 0, err
	}

	xr.br.Reset()
	if _, err := io.Copy(&xr.br, xr.rd); err != nil {
		return 0, 0, err
	}

	// Search for and read the meta block.
	idx := meta.ReverseSearch(xr.br.Bytes())
	if idx < 0 {
		return 0, 0, errCorrupted
	}
	xr.br.Next(idx) // Skip data until magic marker

	xr.bw.Reset()
	xr.mr.Reset(&xr.br)
	if _, err := io.Copy(&xr.bw, &xr.mr); err != nil {
		return 0, 0, errWrap(err)
	}
	if xr.br.Len() != 0 || xr.mr.NumBlocks != 1 {
		return 0, 0, errCorrupted
	}
	if xr.mr.FinalMode != meta.FinalStream {
		return 0, 0, errCorrupted
	}
	if _, err := xr.rd.Seek(-xr.mr.InputOffset, io.SeekCurrent); err != nil {
		return 0, 0, err
	}

	// Parse the footer.
	bufRaw := xr.bw.Bytes()
	if len(bufRaw) < 3 || !bytes.Equal(bufRaw[:3], magic[:]) {
		return 0, 0, errCorrupted // Magic value mismatch
	}
	backSizeU64, cnt := binary.Uvarint(bufRaw[3:])
	if cnt <= 0 {
		return 0, 0, errCorrupted // Integer overflow for VLI
	}
	if len(bufRaw[3+cnt:]) > 0 {
		return 0, 0, errCorrupted // Trailing unread bytes
	}
	return int64(backSizeU64), xr.mr.InputOffset, nil
}
