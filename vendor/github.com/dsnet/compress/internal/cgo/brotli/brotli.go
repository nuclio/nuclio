// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build cgo

// Package brotli implements the Brotli compressed data format using C wrappers.
package brotli

/*
// This relies upon the shared library built from github.com/google/brotli
// at revision 7e347a7c849db05acad20304f5e9b29071ecec7c.
//
// The steps to build and install the shared library is as follows:
//	mkdir out && cd out && ../configure-cmake && make
//	make test
//	make install

#cgo LDFLAGS: -lbrotlidec
#cgo LDFLAGS: -lbrotlienc

#include "brotli/decode.h"
#include "brotli/encode.h"

BrotliDecoderState* zbDecCreate() {
	return BrotliDecoderCreateInstance(NULL, NULL, NULL);
}

BrotliDecoderResult zbDecStream(
	BrotliDecoderState* state,
	size_t* avail_in, const uint8_t* next_in,
	size_t* avail_out, uint8_t* next_out
) {
	return BrotliDecoderDecompressStream(
		state, avail_in, &next_in, avail_out, &next_out, NULL
	);
}

void zbDecDestroy(BrotliDecoderState* state) {
	return BrotliDecoderDestroyInstance(state);
}

BrotliEncoderState* zbEncCreate(int level) {
	BrotliEncoderState* state = BrotliEncoderCreateInstance(NULL, NULL, NULL);
	if (state != NULL) {
		BrotliEncoderSetParameter(state, BROTLI_PARAM_QUALITY, level);
	}
	return state;
}

BROTLI_BOOL zbEncStream(
    BrotliEncoderState* state, BrotliEncoderOperation op,
    size_t* avail_in, const uint8_t* next_in,
    size_t* avail_out, uint8_t* next_out
) {
	return BrotliEncoderCompressStream(
		state, op, avail_in, &next_in, avail_out, &next_out, NULL
	);
}

void zbEncDestroy(BrotliEncoderState* state) {
	return BrotliEncoderDestroyInstance(state);
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
	state *C.BrotliDecoderState
	buf   []byte
	arr   [1 << 14]byte
}

func NewReader(r io.Reader) io.ReadCloser {
	zr := &reader{r: r, state: C.zbDecCreate()}
	if zr.state == nil {
		panic("brotli: could not allocate decoder state")
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
		ret := C.zbDecStream(zr.state, &availIn, ptrIn, &availOut, ptrOut)
		n += len(buf) - int(availOut)
		buf = buf[len(buf)-int(availOut):]
		zr.buf = zr.buf[len(zr.buf)-int(availIn):]

		switch ret {
		case C.BROTLI_DECODER_RESULT_ERROR:
			zr.err = errors.New("brotli: corrupted input")
		case C.BROTLI_DECODER_RESULT_SUCCESS:
			return n, io.EOF
		case C.BROTLI_DECODER_RESULT_NEEDS_MORE_INPUT:
			n1 := copy(zr.arr[:], zr.buf)
			n2, err := zr.r.Read(zr.arr[n1:])
			if n2 > 0 {
				zr.buf = zr.arr[:n1+n2]
			} else if err != nil {
				if err == io.EOF {
					err = io.ErrUnexpectedEOF
				}
				zr.err = err
			}
		case C.BROTLI_DECODER_RESULT_NEEDS_MORE_OUTPUT:
			return n, nil
		default:
			zr.err = errors.New("brotli: unknown decoder error")
		}
	}
	return n, zr.err
}

func (zr *reader) Close() error {
	if zr.state != nil {
		defer func() {
			C.zbDecDestroy(zr.state)
			zr.state = nil
		}()
	}
	return zr.err
}

type writer struct {
	w     io.Writer
	err   error
	state *C.BrotliEncoderState
	buf   []byte
	arr   [1 << 14]byte
}

func NewWriter(w io.Writer, level int) io.WriteCloser {
	if level < C.BROTLI_MIN_QUALITY || level > C.BROTLI_MAX_QUALITY {
		panic("brotli: invalid compression level")
	}

	zw := &writer{w: w, state: C.zbEncCreate(C.int(level))}
	if zw.state == nil {
		panic("brotli: could not allocate encoder state")
	}
	return zw
}

func (zw *writer) Write(buf []byte) (int, error) {
	return zw.write(buf, C.BROTLI_OPERATION_PROCESS)
}

func (zw *writer) write(buf []byte, op C.BrotliEncoderOperation) (int, error) {
	if zw.state == nil {
		return 0, io.ErrClosedPipe
	}

	var n int
	flush := op != C.BROTLI_OPERATION_PROCESS
	for zw.err == nil && (len(buf) > 0 || flush) {
		availIn, availOut, ptrIn, ptrOut := sizePtrs(buf, zw.arr[:])
		ret := C.zbEncStream(zw.state, op, &availIn, ptrIn, &availOut, ptrOut)
		n += len(buf) - int(availIn)
		buf = buf[len(buf)-int(availIn):]
		zw.buf = zw.arr[:len(zw.arr)-int(availOut)]

		if len(zw.buf) > 0 {
			if _, err := zw.w.Write(zw.buf); err != nil {
				zw.err = err
			}
		}
		if ret == 0 && zw.err == nil {
			zw.err = errors.New("brotli: compression error")
		}
		if flush && C.BrotliEncoderHasMoreOutput(zw.state) == 0 {
			break
		}
	}
	return n, zw.err
}

func (zw *writer) Close() error {
	if zw.state != nil {
		defer func() {
			C.zbEncDestroy(zw.state)
			zw.state = nil
		}()
		zw.write(nil, C.BROTLI_OPERATION_FINISH)
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
