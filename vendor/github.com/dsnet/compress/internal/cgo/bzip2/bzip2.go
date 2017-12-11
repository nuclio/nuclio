// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build cgo

// Package bzip2 implements the BZip2 compressed data format using C wrappers.
package bzip2

/*
#cgo LDFLAGS: -lbz2

#include <stdlib.h>
#include "bzlib.h"

bz_stream* bzDecCreate() {
	bz_stream* state = calloc(1, sizeof(bz_stream));
	BZ2_bzDecompressInit(state, 0, 0);
	return state;
}

int bzDecStream(
	bz_stream* state,
	unsigned int* avail_in, char* next_in,
	unsigned int* avail_out, char* next_out
) {
	state->avail_in = *avail_in;
	state->avail_out = *avail_out;
	state->next_in = next_in;
	state->next_out = next_out;
	int ret = BZ2_bzDecompress(state);
	*avail_in = state->avail_in;
	*avail_out = state->avail_out;
	state->next_in = NULL;
	state->next_out = NULL;
	return ret;
}

void bzDecDestroy(bz_stream* state) {
	BZ2_bzDecompressEnd(state);
	free(state);
}

bz_stream* bzEncCreate(int level) {
	bz_stream* state = calloc(1, sizeof(bz_stream));
	BZ2_bzCompressInit(state, level, 0, 0);
	return state;
}

int bzEncStream(
	bz_stream* state, int mode,
	unsigned int* avail_in, char* next_in,
	unsigned int* avail_out, char* next_out
) {
	state->avail_in = *avail_in;
	state->avail_out = *avail_out;
	state->next_in = next_in;
	state->next_out = next_out;
	int ret = BZ2_bzCompress(state, mode);
	*avail_in = state->avail_in;
	*avail_out = state->avail_out;
	state->next_in = NULL;
	state->next_out = NULL;
	return ret;
}

void bzEncDestroy(bz_stream* state) {
	BZ2_bzCompressEnd(state);
	free(state);
}
*/
import "C"

import (
	"errors"
	"io"
	"unsafe"
)

type reader struct {
	r     io.Reader
	err   error
	state *C.bz_stream
	buf   []byte
	arr   [1 << 14]byte
}

func NewReader(r io.Reader) io.ReadCloser {
	zr := &reader{r: r, state: C.bzDecCreate()}
	if zr.state == nil {
		panic("bzip2: could not allocate decoder state")
	}
	return zr
}

func (zr *reader) Read(buf []byte) (int, error) {
	if zr.state == nil {
		return 0, io.ErrClosedPipe
	}

	var n int
	for zr.err == nil && (len(buf) > 0 && n == 0) {
		availIn, availOut, ptrIn, ptrOut := sizePtrs(zr.buf, buf)
		ret := C.bzDecStream(zr.state, &availIn, ptrIn, &availOut, ptrOut)
		n += len(buf) - int(availOut)
		buf = buf[len(buf)-int(availOut):]
		zr.buf = zr.buf[len(zr.buf)-int(availIn):]

		switch ret {
		case C.BZ_OK:
			if len(zr.buf) == 0 && n == 0 {
				n1, err := zr.r.Read(zr.arr[:])
				if n1 > 0 {
					zr.buf = zr.arr[:n1]
				} else if err != nil {
					if err == io.EOF {
						err = io.ErrUnexpectedEOF
					}
					zr.err = err
				}
			}
		case C.BZ_STREAM_END:
			// Handle multi-stream files by re-setting the state.
			if len(zr.buf) == 0 {
				if _, err := io.ReadFull(zr.r, zr.arr[:1]); err != nil {
					if err == io.EOF {
						return n, io.EOF
					}
					zr.err = io.ErrUnexpectedEOF
					return n, zr.err
				}
				zr.buf = zr.arr[:1]
			}
			C.bzDecDestroy(zr.state)
			zr.state = C.bzDecCreate()
		default:
			zr.err = errors.New("bzip2: corrupted input")
		}
	}
	return n, zr.err
}

func (zr *reader) Close() error {
	if zr.state != nil {
		defer func() {
			C.bzDecDestroy(zr.state)
			zr.state = nil
		}()
	}
	return zr.err
}

type writer struct {
	w     io.Writer
	err   error
	state *C.bz_stream
	buf   []byte
	arr   [1 << 14]byte
}

func NewWriter(w io.Writer, level int) io.WriteCloser {
	if level < 1 || level > 9 {
		panic("bzip2: invalid compression level")
	}

	zw := &writer{w: w, state: C.bzEncCreate(C.int(level))}
	if zw.state == nil {
		panic("bzip2: could not allocate encoder state")
	}
	return zw
}

func (zw *writer) Write(buf []byte) (int, error) {
	return zw.write(buf, C.BZ_RUN)
}

func (zw *writer) write(buf []byte, op C.int) (int, error) {
	if zw.state == nil {
		return 0, io.ErrClosedPipe
	}

	var n int
	flush := op != C.BZ_RUN
	for zw.err == nil && (len(buf) > 0 || flush) {
		availIn, availOut, ptrIn, ptrOut := sizePtrs(buf, zw.arr[:])
		ret := C.bzEncStream(zw.state, op, &availIn, ptrIn, &availOut, ptrOut)
		n += len(buf) - int(availIn)
		buf = buf[len(buf)-int(availIn):]
		zw.buf = zw.arr[:len(zw.arr)-int(availOut)]

		if len(zw.buf) > 0 {
			if _, err := zw.w.Write(zw.buf); err != nil {
				zw.err = err
			}
		}
		switch ret {
		case C.BZ_OK, C.BZ_RUN_OK, C.BZ_FLUSH_OK, C.BZ_FINISH_OK:
			continue // Do nothing
		case C.BZ_STREAM_END:
			return n, zw.err
		default:
			zw.err = errors.New("bzip2: compression error")
		}
	}
	return n, zw.err
}

func (zw *writer) Close() error {
	if zw.state != nil {
		defer func() {
			C.bzEncDestroy(zw.state)
			zw.state = nil
		}()
		zw.write(nil, C.BZ_FINISH)
	}
	return zw.err
}

func sizePtrs(in, out []byte) (sizeIn, sizeOut C.uint, ptrIn, ptrOut *C.char) {
	sizeIn = C.uint(len(in))
	sizeOut = C.uint(len(out))
	if len(in) > 0 {
		ptrIn = (*C.char)(unsafe.Pointer(&in[0]))
	}
	if len(out) > 0 {
		ptrOut = (*C.char)(unsafe.Pointer(&out[0]))
	}
	return
}
