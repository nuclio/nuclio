// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package xflate implements the XFLATE compressed data format.
//
// The XFLATE format is a backwards compatible extension to DEFLATE (RFC 1951);
// meaning that any data compressed as XFLATE can also be decompressed by any
// RFC 1951 compliant decoder. XFLATE extends DEFLATE by enabling efficient
// random access reading of the stream. This is accomplished by compressing the
// stream as independent chunks and by encoding an index table at the end of the
// compressed stream. Since the index table contains information about each
// chunk, an XFLATE reader can read this index and determine where to seek to
// in order to satisfy a random access read request.
//
// It is important to remember that all XFLATE streams are DEFLATE streams;
// but, not all DEFLATE streams are XFLATE streams. Only streams written in the
// XFLATE format can be read by xflate.Reader. Thus, do not expect xflate.Reader
// to be able to provide random access to any arbitrary DEFLATE stream.
//
// XFLATE was designed so that current applications of DEFLATE could easily
// switch with few detriments. While XFLATE offers random access reading, it
// does cause the compressed stream to be slightly larger than if the input had
// been compressed as a single DEFLATE stream. The factor that has the most
// effect on the compression overhead is the choice of chunk size.
//
// This compression overhead occurs because XFLATE requires that chunks be
// individually compressed (the LZ77 dictionary is reset before every chunk).
// This means that each chunk cannot benefit from the data that preceded it.
// There is a trade-off in the choice of an appropriate chunk size;
// Smaller chunk sizes allow for better random access properties (since it
// reduces that amount data that a reader may need to discard), but hurts the
// compression ratio. The default chunk size was chosen so that the compression
// overhead was about 1% for most workloads.
//
// Format specification:
//  https://github.com/dsnet/compress/blob/master/doc/xflate-format.pdf
package xflate

import (
	"compress/flate"
	"fmt"

	"github.com/dsnet/compress/internal/errors"
)

// These are the magic values found in the XFLATE footer.
//
// Currently, the flag byte is included as part of the magic since
// all flag bits are currently reserved to be zero.
var magic = [3]byte{'X', 'F', 0x00}

// endBlock is a valid DEFLATE raw block. It is empty and has the final bit set.
// By appending this to any compressed chunk, a normal DEFLATE decompressor can
// be used to read the data.
var endBlock = []byte{0x01, 0x00, 0x00, 0xff, 0xff}

// Writer configuration constants. The values can be set in WriterConfig and
// passed to NewWriter to provide finer granularity control.
const (
	// Compression levels to be used with the underlying DEFLATE compressor.
	NoCompression      = -1
	BestSpeed          = 1
	DefaultCompression = 6
	BestCompression    = 9

	// DefaultChunkSize specifies the default uncompressed size for each chunk.
	// This value was chosen so that the overhead of the XFLATE format would be
	// approximately 1% for the typical dataset.
	DefaultChunkSize = 1 << 18 // 256 KiB

	// DefaultIndexSize specifies the default number of records that each index
	// should contain. This value was chosen so that the maximum amount of
	// memory used for the index is comparable to the memory requirements for
	// DEFLATE compression itself.
	DefaultIndexSize = 1 << 12 // 96 KiB
)

// The FlushMode constants can be passed to Writer.Flush to control the
// specific type of flushing performed.
type FlushMode int

const (
	// FlushSync flushes the write buffer to the underlying writer, but does not
	// reset the dictionary. The intended use case for this is if the underlying
	// writer is a socket or pipe and the caller intends for the recipient to
	// be able to decompress all data written thus far.
	//
	// This is equivalent to SYNC_FLUSH in zlib terminology.
	FlushSync FlushMode = iota

	// FlushFull flushes the write buffer to the underlying writer, and also
	// resets the dictionary. The raw and compressed sizes of the chunk will be
	// inserted into the in-memory index table. The intended use case for this
	// is if the caller wants the current offset to be one that is efficient to
	// seek to by the Reader.
	//
	// Performing this action often can be detrimental to the compression ratio.
	// This is equivalent to FULL_FLUSH in zlib terminology.
	FlushFull

	// FlushIndex flushes the write buffer to the underlying writer, resets the
	// dictionary, write the contents of the current index table, and then
	// clears the index. The intended use case for this is for callers with
	// limited memory to ensure that the index does not grow without limit.
	// Rather than calling this manually, consider using WriterConfig.IndexSize.
	//
	// Performing this action often can be detrimental to the compression ratio
	// and should be use sparingly.
	FlushIndex
)

func errorf(c int, f string, a ...interface{}) error {
	return errors.Error{Code: c, Pkg: "xflate", Msg: fmt.Sprintf(f, a...)}
}

var (
	errCorrupted = errorf(errors.Corrupted, "")
	errClosed    = errorf(errors.Closed, "")
)

func errWrap(err error) error {
	switch err := err.(type) {
	case errors.Error:
		return errorf(err.Code, "%s", err.Msg)
	case flate.CorruptInputError:
		return errCorrupted
	case flate.InternalError:
		return errorf(errors.Internal, "%s", string(err))
	default:
		return err
	}
}
