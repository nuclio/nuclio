// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package xflate

import (
	"bytes"
	"math"
	"reflect"
	"testing"
)

func TestIndexRoundTrip(t *testing.T) {
	vectors := []index{{
		Records:  nil,
		BackSize: 0,
	}, {
		Records:  []record{{10, 41, 1}, {52, 73, 1}, {84, 95, 1}},
		BackSize: 1234,
	}, {
		Records:  []record{{162, 1024, 1}, {325, 2048, 1}, {524, 3072, 1}},
		BackSize: 251,
	}, {
		Records:  []record{{math.MaxInt64, math.MaxInt64, 1}},
		BackSize: math.MaxInt64,
	}, {
		Records:  []record{{5, 0, 1}, {10, 1, 1}, {15, 2, 1}, {math.MaxInt64, math.MaxInt64, 1}},
		BackSize: 1337,
	}}

	for i, idx1 := range vectors {
		var idx2 index
		bb := new(bytes.Buffer)

		// Write the encoded index.
		var xw Writer
		xw.wr = bb
		if err := xw.encodeIndex(&idx1); err != nil {
			t.Errorf("test %d, unexpected error: encodeIndex() = %v", i, err)
		}

		// Read the encoded index.
		var xr Reader
		xr.rd = bytes.NewReader(bb.Bytes())
		idx2.IndexSize = idx1.IndexSize
		if err := xr.decodeIndex(&idx2); err != nil {
			t.Errorf("test %d, unexpected error: decodeIndex() = %v", i, err)
		}

		if !reflect.DeepEqual(idx1, idx2) {
			t.Errorf("test %d, mismatching indexes:\ngot  %v\nwant %v", i, idx2, idx1)
		}
	}
}

func TestIndex(t *testing.T) {
	var idx index

	// Empty index.
	if idx.LastRecord() != (record{}) {
		t.Errorf("last record mismatch: got %v, want %v", idx.LastRecord(), record{})
	}

	// Append entries.
	recs := []struct {
		csize, rsize int64
		ok           bool
	}{
		{0, 0, true},
		{3, 5, true},
		{31, 62, true},
		{-1, 6, false},
		{6, 13, true},
		{math.MaxInt64, 3, false},
	}
	for _, v := range recs {
		if ok := idx.AppendRecord(v.csize, v.rsize, 0); ok != v.ok {
			t.Errorf("unexpected result: AppendRecord(%d, %d) = %v, want %v", v.csize, v.rsize, ok, v.ok)
		}
	}
	if want := (record{40, 80, 0}); idx.LastRecord() != want {
		t.Errorf("last record mismatch: got %v, want %v", idx.LastRecord(), want)
	}

	// Append indexes.
	idxs := []struct {
		idx index
		ok  bool
	}{
		{index{Records: []record{}}, true},
		{index{Records: []record{{1, 4, 1}, {3, 9, 2}, {13, 153, 3}}}, true},
		{index{Records: []record{{1, 4, 4}, {3, 9, 5}, {13, 8, 6}}}, false},
	}
	for _, v := range idxs {
		if ok := idx.AppendIndex(&v.idx); ok != v.ok {
			t.Errorf("unexpected result: AppendIndex(%v) = %v, want %v", v.idx, ok, v.ok)
		}
	}
	if want := (record{53, 233, 3}); idx.LastRecord() != want {
		t.Errorf("last record mismatch: got %v, want %v", idx.LastRecord(), want)
	}

	// Final check.
	want := index{Records: []record{
		{0, 0, 0}, {3, 5, 0}, {34, 67, 0}, {40, 80, 0}, {41, 84, 1}, {43, 89, 2}, {53, 233, 3},
	}}
	if !reflect.DeepEqual(idx, want) {
		t.Errorf("mismatching index:\ngot  %v\nwant %v", idx, want)
	}
}

func TestIndexSearch(t *testing.T) {
	type query struct {
		offset     int64  // Input query
		prev, curr record // Expected output
	}
	vectors := []struct {
		idx index   // The index to query on
		qs  []query // A list of query results
	}{{
		idx: index{},
		qs: []query{
			{0, record{}, record{}},
			{5, record{}, record{}},
		},
	}, {
		idx: index{Records: []record{{2, 14, 0}}},
		qs: []query{
			{0, record{0, 0, 0}, record{2, 14, 0}},
			{5, record{0, 0, 0}, record{2, 14, 0}},
			{13, record{0, 0, 0}, record{2, 14, 0}},
			{14, record{2, 14, 0}, record{2, 14, 0}},
			{15, record{2, 14, 0}, record{2, 14, 0}},
		},
	}, {
		idx: index{Records: []record{{2, 14, 0}, {3, 17, 0}}},
		qs: []query{
			{0, record{0, 0, 0}, record{2, 14, 0}},
			{5, record{0, 0, 0}, record{2, 14, 0}},
			{13, record{0, 0, 0}, record{2, 14, 0}},
			{14, record{2, 14, 0}, record{3, 17, 0}},
			{15, record{2, 14, 0}, record{3, 17, 0}},
			{16, record{2, 14, 0}, record{3, 17, 0}},
			{17, record{3, 17, 0}, record{3, 17, 0}},
			{18, record{3, 17, 0}, record{3, 17, 0}},
		},
	}, {
		idx: index{Records: []record{{2, 14, 0}, {2, 14, 0}}},
		qs: []query{
			{0, record{0, 0, 0}, record{2, 14, 0}},
			{13, record{0, 0, 0}, record{2, 14, 0}},
			{14, record{2, 14, 0}, record{2, 14, 0}},
			{15, record{2, 14, 0}, record{2, 14, 0}},
		},
	}, {
		idx: index{Records: []record{
			{17, 5, 0}, {30, 8, 0}, {41, 9, 0}, {53, 11, 0}, {66, 12, 0}, {80, 12, 0},
			{95, 16, 0}, {111, 16, 0}, {128, 16, 0}, {146, 16, 0}, {165, 16, 0},
			{185, 16, 0}, {206, 19, 0}, {228, 21, 0}, {251, 22, 0}, {275, 22, 0},
		}},
		qs: []query{
			{0, record{0, 0, 0}, record{17, 5, 0}},
			{9, record{41, 9, 0}, record{53, 11, 0}},
			{10, record{41, 9, 0}, record{53, 11, 0}},
			{11, record{53, 11, 0}, record{66, 12, 0}},
			{15, record{80, 12, 0}, record{95, 16, 0}},
			{16, record{185, 16, 0}, record{206, 19, 0}},
			{17, record{185, 16, 0}, record{206, 19, 0}},
			{22, record{275, 22, 0}, record{275, 22, 0}},
			{100, record{275, 22, 0}, record{275, 22, 0}},
		},
	}}

	for i, v := range vectors {
		for j, q := range v.qs {
			prev, curr := v.idx.GetRecords(v.idx.Search(q.offset))
			if prev != q.prev || curr != q.curr {
				t.Errorf("test %d, query %d, search result mismatch: Search(%d) = (%v %v), want (%v %v)",
					i, j, q.offset, prev, curr, q.prev, q.curr)
			}
		}
	}
}
