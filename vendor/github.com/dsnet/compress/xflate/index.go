// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package xflate

const (
	unknownType = iota
	deflateType
	indexType
	footerType
)

type index struct {
	// Records is a list of records that indicate the location of all chunks
	// in the stream. However, rather than recording the starting offset of
	// each chunk, only the ending offsets are recorded.
	//
	// The starting record {0, 0} is not included since it is implied.
	// The last record effectively holds the total size of the stream.
	Records []record

	BackSize  int64 // Size of previous index when encoded
	IndexSize int64 // Size of this index when encoded
}

type record struct {
	CompOffset int64 // Offset in compressed stream where decompression can start from
	RawOffset  int64 // The uncompressed offset that CompOffset is associated with
	Type       int   // Type of the record
}

// Reset resets the index.
func (idx *index) Reset() {
	*idx = index{Records: idx.Records[:0]}
}

// AppendRecord appends a new record to the end of the index and reports whether
// the operation was successful or not.
func (idx *index) AppendRecord(compSize, rawSize int64, typ int) bool {
	if rawSize < 0 || compSize < 0 {
		return false // Invalid size
	}

	lastRec := idx.LastRecord()
	rec := record{
		CompOffset: lastRec.CompOffset + compSize,
		RawOffset:  lastRec.RawOffset + rawSize,
		Type:       typ,
	}
	if rec.CompOffset < lastRec.CompOffset || rec.RawOffset < lastRec.RawOffset {
		return false // Overflow detected
	}
	idx.Records = append(idx.Records, rec)
	return true
}

// AppendIndex appends the contents of another index onto the current receiver
// and reports whether the operation was successful or not.
func (idx *index) AppendIndex(other *index) bool {
	var preRec record
	for i, rec := range other.Records {
		csize, rsize := rec.CompOffset-preRec.CompOffset, rec.RawOffset-preRec.RawOffset
		if !idx.AppendRecord(csize, rsize, rec.Type) {
			idx.Records = idx.Records[:len(idx.Records)-i] // Ensure atomic append
			return false
		}
		preRec = rec
	}
	return true
}

// Search searches for the record that best matches the raw offset given.
// This search will return the location of the record with the lowest
// RawOffset that is still greater than the given offset.
// It return -1 if such a record does not exist.
//
// This method is intended to be used in conjunction with GetRecords,
// which returns a pair of records (prev, curr).
// With these records, the following can be computed:
//
//	// Where in the underlying reader the decompressor should start from.
//	compOffset := prev.CompOffset
//
//	// The total number of uncompressed bytes to discard to reach offset.
//	rawDiscard := offset - prev.RawOffset
//
//	// The total compressed size of the current block.
//	compSize := curr.CompOffset - prev.CompOffset
//
//	// The total uncompressed size of the current block.
//	rawSize := curr.RawOffset - prev.RawOffset
//
func (idx *index) Search(offset int64) int {
	recs := idx.Records
	i, imin, imax := -1, 0, len(recs)-1
	for imax >= imin && i == -1 {
		imid := (imin + imax) / 2
		gteCurr := bool(offset >= recs[imid].RawOffset)
		ltNext := bool(imid+1 >= len(recs) || offset < recs[imid+1].RawOffset)
		if gteCurr && ltNext {
			i = imid
		} else if gteCurr {
			imin = imid + 1
		} else {
			imax = imid - 1
		}
	}
	return i + 1
}

// GetRecords returns the previous and current records at the given position.
// This method will automatically bind the search position within the bounds
// of the index. Thus, this will return zero value records if the position is
// too low, and the last record if the value is too high.
func (idx *index) GetRecords(i int) (prev, curr record) {
	recs := idx.Records
	if i > len(recs) {
		i = len(recs)
	}
	if i-1 >= 0 && i-1 < len(recs) {
		prev = recs[i-1]
	}
	if i >= 0 && i < len(recs) {
		curr = recs[i]
	} else {
		curr = prev
		curr.Type = unknownType
	}
	return prev, curr
}

// LastRecord returns the last record if it exists, otherwise the zero value.
func (idx *index) LastRecord() record {
	var rec record
	if len(idx.Records) > 0 {
		rec = idx.Records[len(idx.Records)-1]
	}
	return rec
}
