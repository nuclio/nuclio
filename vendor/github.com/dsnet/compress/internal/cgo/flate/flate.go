// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build cgo

// Package flate implements the DEFLATE compressed data format,
// described in RFC 1951, using C wrappers.
package flate

/*
#cgo LDFLAGS: -lz

#include <stdlib.h>
#include "zlib.h"

z_streamp zfDecCreate() {
	z_streamp state = calloc(1, sizeof(z_stream));
	inflateInit2(state, -MAX_WBITS);
	return state;
}

int zfDecStream(
	z_streamp state,
	uInt* avail_in, Bytef* next_in,
	uInt* avail_out, Bytef* next_out
) {
	state->avail_in = *avail_in;
	state->avail_out = *avail_out;
	state->next_in = next_in;
	state->next_out = next_out;
	int ret = inflate(state, Z_NO_FLUSH);
	*avail_in = state->avail_in;
	*avail_out = state->avail_out;
	state->next_in = NULL;
	state->next_out = NULL;
	return ret;
}

void zfDecDestroy(z_streamp state) {
	inflateEnd(state);
	free(state);
}

z_streamp zfEncCreate(int level) {
	z_streamp state = calloc(1, sizeof(z_stream));
	deflateInit2(state, level, Z_DEFLATED, -MAX_WBITS, MAX_MEM_LEVEL, Z_DEFAULT_STRATEGY);
	return state;
}

int zfEncStream(
	z_streamp state, int flush,
	uInt* avail_in, Bytef* next_in,
	uInt* avail_out, Bytef* next_out
) {
	state->avail_in = *avail_in;
	state->avail_out = *avail_out;
	state->next_in = next_in;
	state->next_out = next_out;
	int ret = deflate(state, flush);
	*avail_in = state->avail_in;
	*avail_out = state->avail_out;
	state->next_in = NULL;
	state->next_out = NULL;
	return ret;
}

void zfEncDestroy(z_streamp state) {
	deflateEnd(state);
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
	state C.z_streamp
	buf   []byte
	arr   [1 << 14]byte
}

func NewReader(r io.Reader) io.ReadCloser {
	zr := &reader{r: r, state: C.zfDecCreate()}
	if zr.state == nil {
		panic("flate: could not allocate decoder state")
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
		ret := C.zfDecStream(zr.state, &availIn, ptrIn, &availOut, ptrOut)
		n += len(buf) - int(availOut)
		buf = buf[len(buf)-int(availOut):]
		zr.buf = zr.buf[len(zr.buf)-int(availIn):]

		switch ret {
		case C.Z_OK:
			return n, nil
		case C.Z_BUF_ERROR:
			if len(zr.buf) == 0 {
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
		case C.Z_STREAM_END:
			return n, io.EOF
		default:
			zr.err = errors.New("flate: corrupted input")
		}
	}
	return n, zr.err
}

func (zr *reader) Close() error {
	if zr.state != nil {
		defer func() {
			C.zfDecDestroy(zr.state)
			zr.state = nil
		}()
	}
	return zr.err
}

type writer struct {
	w     io.Writer
	err   error
	state C.z_streamp
	buf   []byte
	arr   [1 << 14]byte
}

func NewWriter(w io.Writer, level int) io.WriteCloser {
	if level < C.Z_NO_COMPRESSION || level > C.Z_BEST_COMPRESSION {
		panic("flate: invalid compression level")
	}

	zw := &writer{w: w, state: C.zfEncCreate(C.int(level))}
	if zw.state == nil {
		panic("flate: could not allocate encoder state")
	}
	return zw
}

func (zw *writer) Write(buf []byte) (int, error) {
	return zw.write(buf, C.Z_NO_FLUSH)
}

func (zw *writer) write(buf []byte, op C.int) (int, error) {
	if zw.state == nil {
		return 0, io.ErrClosedPipe
	}

	var n int
	flush := op != C.Z_NO_FLUSH
	for zw.err == nil && (len(buf) > 0 || flush) {
		availIn, availOut, ptrIn, ptrOut := sizePtrs(buf, zw.arr[:])
		ret := C.zfEncStream(zw.state, op, &availIn, ptrIn, &availOut, ptrOut)
		n += len(buf) - int(availIn)
		buf = buf[len(buf)-int(availIn):]
		zw.buf = zw.arr[:len(zw.arr)-int(availOut)]

		if len(zw.buf) > 0 {
			if _, err := zw.w.Write(zw.buf); err != nil {
				zw.err = err
			}
		}
		switch ret {
		case C.Z_OK, C.Z_BUF_ERROR:
			continue // Do nothing
		case C.Z_STREAM_END:
			return n, zw.err
		default:
			zw.err = errors.New("flate: compression error")
		}
	}
	return n, zw.err
}

func (zw *writer) Close() error {
	if zw.state != nil {
		defer func() {
			C.zfEncDestroy(zw.state)
			zw.state = nil
		}()
		zw.write(nil, C.Z_FINISH)
	}
	return zw.err
}

func sizePtrs(in, out []byte) (sizeIn, sizeOut C.uInt, ptrIn, ptrOut *C.Bytef) {
	sizeIn = C.uInt(len(in))
	sizeOut = C.uInt(len(out))
	if len(in) > 0 {
		ptrIn = (*C.Bytef)(unsafe.Pointer(&in[0]))
	}
	if len(out) > 0 {
		ptrOut = (*C.Bytef)(unsafe.Pointer(&out[0]))
	}
	return
}
