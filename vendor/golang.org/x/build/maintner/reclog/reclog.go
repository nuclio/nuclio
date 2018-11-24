// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package reclog contains readers and writers for a record wrapper
// format used by maintner.
package reclog

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
)

// The reclog format is as follows:
//
// The log is a series of binary blobs. Each record begins with the
// variably-lengthed prefix "REC@XXX+YYY=" where the 0+ XXXX digits
// are the hex offset on disk (where the 'R' on disk is written) and
// the 0+ YYY digits are the hex length of the blob. After the YYY
// digits there is a '=' byte before the YYY bytes of blob. There is
// no record footer.
var (
	headerPrefix = []byte("REC@")
	headerSuffix = []byte("=")
	plus         = []byte("+")
)

// RecordCallback is the callback signature accepted by
// ForeachFileRecord and ForeachRecord, which read the mutation log
// format used by DiskMutationLogger.
//
// Offset is the offset in the logical of physical file.
// hdr and bytes are only valid until the function returns
// and must not be retained.
//
// hdr is the record header, in the form "REC@c765c9a+1d3=" (REC@ <hex
// offset> + <hex len(rec)> + '=').
//
// rec is the proto3 binary marshalled representation of
// *maintpb.Mutation.
//
// If the callback returns an error, iteration stops.
type RecordCallback func(off int64, hdr, rec []byte) error

// ForeachFileRecord calls fn for each record in the named file.
// Calls to fn are made serially.
// If fn returns an error, iteration ends and that error is returned.
func ForeachFileRecord(path string, fn RecordCallback) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := ForeachRecord(f, 0, fn); err != nil {
		return fmt.Errorf("error in %s: %v", path, err)
	}
	return nil
}

// ForeachRecord calls fn for each record in r.
// Calls to fn are made serially.
// If fn returns an error, iteration ends and that error is returned.
// The startOffset be 0 if reading from the beginning of a file.
func ForeachRecord(r io.Reader, startOffset int64, fn RecordCallback) error {
	off := startOffset
	br := bufio.NewReader(r)
	var buf bytes.Buffer
	var hdrBuf bytes.Buffer
	for {
		startOff := off
		hdr, err := br.ReadSlice('=')
		if err != nil {
			if err == io.EOF && len(hdr) == 0 {
				return nil
			}
			return err
		}
		if len(hdr) > 40 {
			return fmt.Errorf("malformed overlong header %q at offset %v", hdr[:40], startOff)
		}
		hdrBuf.Reset()
		hdrBuf.Write(hdr)
		if !bytes.HasPrefix(hdr, headerPrefix) || !bytes.HasSuffix(hdr, headerSuffix) || bytes.Count(hdr, plus) != 1 {
			return fmt.Errorf("malformed header %q at offset %v", hdr, startOff)
		}
		plusPos := bytes.IndexByte(hdr, '+')
		hdrOff, err := strconv.ParseInt(string(hdr[len(headerPrefix):plusPos]), 16, 64)
		if err != nil {
			return fmt.Errorf("malformed header %q (malformed offset) at offset %v", hdr, startOff)
		}
		if hdrOff != startOff {
			return fmt.Errorf("malformed header %q with offset %v doesn't match expected offset %v", hdr, hdrOff, startOff)
		}
		hdrSize, err := strconv.ParseInt(string(hdr[plusPos+1:len(hdr)-1]), 16, 64)
		if err != nil {
			return fmt.Errorf("malformed header %q (bad size) at offset %v", hdr, startOff)
		}
		off += int64(len(hdr))

		buf.Reset()
		if _, err := io.CopyN(&buf, br, hdrSize); err != nil {
			return fmt.Errorf("truncated record at offset %v: %v", startOff, err)
		}
		off += hdrSize
		if err := fn(startOff, hdrBuf.Bytes(), buf.Bytes()); err != nil {
			return err
		}
	}
}

// AppendRecordToFile opens the named filename for append (creating it
// if necessary) and adds the provided data record to the end.
// The caller is responsible for file locking.
func AppendRecordToFile(filename string, data []byte) error {
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	off, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if off != st.Size() {
		return fmt.Errorf("Size %v != offset %v", st.Size(), off)
	}
	if err := WriteRecord(f, off, data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// WriteRecord writes the record data to w, formatting the record
// wrapper with the given offset off. It is the caller's
// responsibility to pass the correct offset. Exactly one Write
// call will be made to w.
func WriteRecord(w io.Writer, off int64, data []byte) error {
	_, err := fmt.Fprintf(w, "REC@%x+%x=%s", off, len(data), data)
	return err
}
