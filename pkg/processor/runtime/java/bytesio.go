/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package java

import (
	"fmt"
)

// BytesIO implements io.Reader & io.Writer over a []byte
type BytesIO struct {
	buf    []byte
	offset int
	size   int
}

// NewBytesIO returns a new BytesIO over buf
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

// Seek sets the buffer location to n, trimming is needed
func (bio *BytesIO) Seek(n int) {
	if n >= bio.size {
		n = bio.size - 1
	}

	if n < 0 {
		n = 0
	}

	bio.offset = n
}

// Reset resets the location and the size of the buffer
func (bio *BytesIO) Reset() {
	bio.Seek(0)
	bio.size = 0
}
