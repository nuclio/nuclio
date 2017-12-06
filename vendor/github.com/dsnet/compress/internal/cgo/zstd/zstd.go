// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build cgo

// Package zstd implements the Zstandard compressed data format using C wrappers.
package zstd

/*
// This relies upon the shared library built from github.com/facebook/zstd
// at revision 39c105c60589cd58714afa0be3d0e397101993f5.
//
// The steps to build and install the shared library is as follows:
//	make install
//	make test

#cgo LDFLAGS: -lzstd

#include <stdlib.h>
#include <stdint.h>
#include "zstd.h"

ZSTD_DStream* zsDecCreate() {
	ZSTD_DStream* state = ZSTD_createDStream();
	ZSTD_initDStream(state);
	return state;
}

size_t zsDecStream(
	ZSTD_DStream* state,
	size_t* avail_in, uint8_t* next_in,
	size_t* avail_out, uint8_t* next_out
) {
	ZSTD_inBuffer in = {next_in, *avail_in, 0};
	ZSTD_outBuffer out = {next_out, *avail_out, 0};
	size_t ret = ZSTD_decompressStream(state, &out, &in);
	*avail_in = in.size - in.pos;
	*avail_out = out.size - out.pos;
	in.src = NULL;
	out.dst = NULL;
	return ret;
}

void zsDecDestroy(ZSTD_DStream* state) {
	ZSTD_freeDStream(state);
}

ZSTD_CStream* zsEncCreate(int level) {
	ZSTD_CStream* state = ZSTD_createCStream();
	ZSTD_initCStream(state, level);
	return state;
}

size_t zsEncStream(
	ZSTD_CStream* state, int finish,
	size_t* avail_in, uint8_t* next_in,
	size_t* avail_out, uint8_t* next_out
) {
	ZSTD_inBuffer in = {next_in, *avail_in, 0};
	ZSTD_outBuffer out = {next_out, *avail_out, 0};
	size_t ret = finish ?
		ZSTD_endStream(state, &out) : ZSTD_compressStream(state, &out, &in);
	*avail_in = in.size - in.pos;
	*avail_out = out.size - out.pos;
	in.src = NULL;
	out.dst = NULL;
	return ret;
}

void zsEncDestroy(ZSTD_CStream* state) {
	ZSTD_freeCStream(state);
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
	state *C.ZSTD_DStream
	buf   []byte
	arr   [1 << 14]byte
}

func NewReader(r io.Reader) io.ReadCloser {
	zr := &reader{r: r, state: C.zsDecCreate()}
	if zr.state == nil {
		panic("zstd: could not allocate decoder state")
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
		ret := C.zsDecStream(zr.state, &availIn, ptrIn, &availOut, ptrOut)
		n += len(buf) - int(availOut)
		buf = buf[len(buf)-int(availOut):]
		zr.buf = zr.buf[len(zr.buf)-int(availIn):]

		switch {
		case C.ZSTD_isError(ret) > 0:
			zr.err = errors.New("zstd: corrupted input")
		case ret == 0:
			return n, io.EOF
		case n > 0:
			return n, nil
		case len(zr.buf) == 0 && n == 0:
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
	}
	return n, zr.err
}

func (zr *reader) Close() error {
	if zr.state != nil {
		defer func() {
			C.zsDecDestroy(zr.state)
			zr.state = nil
		}()
	}
	return zr.err
}

type writer struct {
	w     io.Writer
	err   error
	state *C.ZSTD_CStream
	buf   []byte
	arr   [1 << 14]byte
}

func NewWriter(w io.Writer, level int) io.WriteCloser {
	if level < 1 || level > 22 {
		panic("zstd: invalid compression level")
	}

	zw := &writer{w: w, state: C.zsEncCreate(C.int(level))}
	if zw.state == nil {
		panic("zstd: could not allocate encoder state")
	}
	return zw
}

func (zw *writer) Write(buf []byte) (int, error) {
	return zw.write(buf, 0)
}

func (zw *writer) write(buf []byte, finish C.int) (int, error) {
	if zw.state == nil {
		return 0, io.ErrClosedPipe
	}

	var n int
	for zw.err == nil && (len(buf) > 0 || finish > 0) {
		availIn, availOut, ptrIn, ptrOut := sizePtrs(buf, zw.arr[:])
		ret := C.zsEncStream(zw.state, finish, &availIn, ptrIn, &availOut, ptrOut)
		n += len(buf) - int(availIn)
		buf = buf[len(buf)-int(availIn):]
		zw.buf = zw.arr[:len(zw.arr)-int(availOut)]

		if len(zw.buf) > 0 {
			if _, err := zw.w.Write(zw.buf); err != nil {
				zw.err = err
			}
		}
		switch {
		case C.ZSTD_isError(ret) > 0:
			zw.err = errors.New("zstd: compression error")
		case len(buf) == 0 && len(zw.buf) == 0:
			return n, zw.err
		case ret == 0 && finish > 0:
			return n, zw.err
		}
	}
	return n, zw.err
}

func (zw *writer) Close() error {
	if zw.state != nil {
		defer func() {
			C.zsEncDestroy(zw.state)
			zw.state = nil
		}()
		zw.write(nil, 1)
	}
	return zw.err
}

func sizePtrs(in, out []byte) (sizeIn, sizeOut C.size_t, ptrIn, ptrOut *C.uint8_t) {
	sizeIn = C.size_t(len(in))
	sizeOut = C.size_t(len(out))
	if len(in) > 0 {
		ptrIn = (*C.uint8_t)(unsafe.Pointer(&in[0]))
	}
	if len(out) > 0 {
		ptrOut = (*C.uint8_t)(unsafe.Pointer(&out[0]))
	}
	return
}
