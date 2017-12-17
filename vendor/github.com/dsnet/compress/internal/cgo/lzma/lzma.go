// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build cgo

// Package lzma implements the LZMA2 compressed data format using C wrappers.
package lzma

/*
#cgo LDFLAGS: -llzma

#include <assert.h>
#include <stdlib.h>
#include "lzma.h"

// zlState is a tuple of C allocated data structures.
//
// The liblzma documentation is not clear about whether the filters struct must
// stay live past calls to lzma_raw_encoder and lzma_raw_decoder.
// To be on the safe side, we allocate them and keep them around until the end.
typedef struct {
	lzma_stream stream;
	lzma_filter filters[2];
	lzma_options_lzma options;
} zlState;

zlState* zlDecCreate() {
	zlState* state = calloc(1, sizeof(zlState));
	state->filters[0].id = LZMA_FILTER_LZMA2;
	state->filters[0].options = &state->options;
	state->filters[1].id = LZMA_VLI_UNKNOWN;
	state->options.dict_size = LZMA_DICT_SIZE_DEFAULT;

	assert(lzma_raw_decoder(&state->stream, state->filters) == LZMA_OK);
	return state;
}

zlState* zlEncCreate(int level) {
	zlState* state = calloc(1, sizeof(zlState));
	state->filters[0].id = LZMA_FILTER_LZMA2;
	state->filters[0].options = &state->options;
	state->filters[1].id = LZMA_VLI_UNKNOWN;

	assert(!lzma_lzma_preset(&state->options, level));
	assert(lzma_raw_encoder(&state->stream, state->filters) == LZMA_OK);
	return state;
}

lzma_ret zlStream(
	lzma_stream* strm, lzma_action action,
	size_t* avail_in, uint8_t* next_in,
	size_t* avail_out, uint8_t* next_out
) {
	strm->avail_in = *avail_in;
	strm->avail_out = *avail_out;
	strm->next_in = next_in;
	strm->next_out = next_out;
	lzma_ret ret = lzma_code(strm, action);
	*avail_in = strm->avail_in;
	*avail_out = strm->avail_out;
	strm->next_in = NULL;
	strm->next_out = NULL;
	return ret;
}

void zlDestroy(zlState* state) {
	lzma_end(&state->stream);
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
	state *C.zlState
	buf   []byte
	arr   [1 << 14]byte
}

func NewReader(r io.Reader) io.ReadCloser {
	zr := &reader{r: r, state: C.zlDecCreate()}
	if zr.state == nil {
		panic("lzma: could not allocate decoder state")
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
		ret := C.zlStream(&zr.state.stream, 0, &availIn, ptrIn, &availOut, ptrOut)
		n += len(buf) - int(availOut)
		buf = buf[len(buf)-int(availOut):]
		zr.buf = zr.buf[len(zr.buf)-int(availIn):]

		switch ret {
		case C.LZMA_OK:
			return n, nil
		case C.LZMA_BUF_ERROR:
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
		case C.LZMA_STREAM_END:
			return n, io.EOF
		default:
			zr.err = errors.New("lzma: corrupted input")
		}
	}
	return n, zr.err
}

func (zr *reader) Close() error {
	if zr.state != nil {
		defer func() {
			C.zlDestroy(zr.state)
			zr.state = nil
		}()
	}
	return zr.err
}

type writer struct {
	w     io.Writer
	err   error
	state *C.zlState
	buf   []byte
	arr   [1 << 14]byte
}

func NewWriter(w io.Writer, level int) io.WriteCloser {
	if level < 0 || level > 9 {
		panic("lzma: invalid compression level")
	}

	zw := &writer{w: w, state: C.zlEncCreate(C.int(level))}
	if zw.state == nil {
		panic("lzma: could not allocate encoder state")
	}
	return zw
}

func (zw *writer) Write(buf []byte) (int, error) {
	return zw.write(buf, C.LZMA_RUN)
}

func (zw *writer) write(buf []byte, op C.lzma_action) (int, error) {
	if zw.state == nil {
		return 0, io.ErrClosedPipe
	}

	var n int
	flush := op != C.LZMA_RUN
	for zw.err == nil && (len(buf) > 0 || flush) {
		availIn, availOut, ptrIn, ptrOut := sizePtrs(buf, zw.arr[:])
		ret := C.zlStream(&zw.state.stream, op, &availIn, ptrIn, &availOut, ptrOut)
		n += len(buf) - int(availIn)
		buf = buf[len(buf)-int(availIn):]
		zw.buf = zw.arr[:len(zw.arr)-int(availOut)]

		if len(zw.buf) > 0 {
			if _, err := zw.w.Write(zw.buf); err != nil {
				zw.err = err
			}
		}
		switch ret {
		case C.LZMA_OK, C.LZMA_BUF_ERROR:
			continue // Do nothing
		case C.LZMA_STREAM_END:
			return n, zw.err
		default:
			zw.err = errors.New("lzma: compression error")
		}
	}
	return n, zw.err
}

func (zw *writer) Close() error {
	if zw.state != nil {
		defer func() {
			C.zlDestroy(zw.state)
			zw.state = nil
		}()
		zw.write(nil, C.LZMA_FINISH)
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
