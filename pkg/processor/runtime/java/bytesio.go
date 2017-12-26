package main

import (
	"fmt"
)

type BytesIO struct {
	buf    []byte
	offset int
	size   int
}

func NewBytesIO(buf []byte) *BytesIO {
	return &BytesIO{buf, 0, 0}
}

func (bio *BytesIO) Write(p []byte) (int, error) {
	if len(p)+bio.offset > len(bio.buf) {
		return 0, fmt.Errorf("size too big (%d > %d)", len(p)+bio.offset, len(bio.buf))
	}

	copy(bio.buf[bio.offset:], p)
	bio.offset += len(p)
	bio.size = bio.offset
	return len(p), nil
}

func (bio *BytesIO) Read(p []byte) (int, error) {
	n := len(p)
	if bio.offset+n > bio.size {
		n = bio.size - bio.offset
	}
	copy(p, bio.buf[bio.offset:bio.offset+n])
	bio.offset += n
	return n, nil
}

func (bio *BytesIO) Seek(n int) {
	if n >= bio.size {
		n = bio.size - 1
	}

	if n < 0 {
		n = 0
	}

	bio.offset = n
}

func (bio *BytesIO) Reset() {
	bio.Seek(0)
	bio.size = 0
}
