// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build gofuzz

// This file exists to export internal implementation details for fuzz testing.

package xflate

import (
	"io"

	"github.com/dsnet/compress/xflate/internal/meta"
)

func NewMetaReader(r io.Reader) *meta.Reader {
	return meta.NewReader(r)
}

func NewMetaWriter(r io.Writer) *meta.Writer {
	return meta.NewWriter(r)
}
