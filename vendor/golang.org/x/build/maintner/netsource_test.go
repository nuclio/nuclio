// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package maintner

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

func TestSumSegSize(t *testing.T) {
	tests := []struct {
		in   []fileSeg
		want int64
	}{
		{
			in:   []fileSeg{fileSeg{size: 1}},
			want: 1,
		},
		{
			in:   []fileSeg{fileSeg{size: 1}, fileSeg{size: 100}},
			want: 101,
		},
		{
			in:   nil,
			want: 0,
		},
	}
	for i, tt := range tests {
		got := sumSegSize(tt.in)
		if got != tt.want {
			t.Errorf("%d. sumSegSize = %v; want %v", i, got, tt.want)
		}
	}
}

func TestSumCommonPrefixSize(t *testing.T) {
	tests := []struct {
		a, b   []fileSeg
		summer func(file string, n int64) string
		want   int64
	}{
		{
			a:    []fileSeg{fileSeg{size: 1, sha224: "abab"}},
			b:    []fileSeg{fileSeg{size: 1, sha224: "abab"}},
			want: 1,
		},
		{
			a:    []fileSeg{fileSeg{size: 1, sha224: "abab"}},
			b:    []fileSeg{fileSeg{size: 1, sha224: "eeee"}},
			want: 0,
		},
		{
			a: []fileSeg{
				fileSeg{size: 100, sha224: "abab"},
				fileSeg{size: 100, sha224: "abab", file: "a.mutlog"},
			},
			b: []fileSeg{
				fileSeg{size: 100, sha224: "abab"},
				fileSeg{size: 50, sha224: "cccc"},
			},
			summer: func(file string, n int64) string {
				if file == "a.mutlog" && n == 50 {
					return "cccc"
				}
				return "xxx"
			},
			want: 150,
		},
		{
			a: []fileSeg{
				fileSeg{size: 100, sha224: "abab"},
				fileSeg{size: 50, sha224: "cccc"},
			},
			b: []fileSeg{
				fileSeg{size: 100, sha224: "abab"},
				fileSeg{size: 100, sha224: "abab", file: "b.mutlog"},
			},
			summer: func(file string, n int64) string {
				if file == "b.mutlog" && n == 50 {
					return "cccc"
				}
				return "xxx"
			},
			want: 150,
		},
	}
	for i, tt := range tests {
		summer := tt.summer
		if summer == nil {
			summer = func(file string, n int64) string {
				t.Errorf("%d. unexpected call to prefix summer for file=%q, n=%v", i, file, n)
				return ""
			}
		}
		ns := &netMutSource{
			testHookFilePrefixSum224: summer,
		}
		got := ns.sumCommonPrefixSize(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("%d. sumCommonPrefixSize = %v; want %v", i, got, tt.want)
		}
	}
}

func TestTrimLeadingSegBytes(t *testing.T) {
	tests := []struct {
		in   []fileSeg
		trim int64
		want []fileSeg
	}{
		{
			in:   []fileSeg{fileSeg{size: 100}, fileSeg{size: 50}},
			trim: 0,
			want: []fileSeg{fileSeg{size: 100}, fileSeg{size: 50}},
		},
		{
			in:   []fileSeg{fileSeg{size: 100}, fileSeg{size: 50}},
			trim: 150,
			want: nil,
		},
		{
			in:   []fileSeg{fileSeg{size: 100}, fileSeg{size: 50}},
			trim: 100,
			want: []fileSeg{fileSeg{size: 50}},
		},
		{
			in:   []fileSeg{fileSeg{size: 100}, fileSeg{size: 50}},
			trim: 25,
			want: []fileSeg{fileSeg{size: 100, skip: 25}, fileSeg{size: 50}},
		},
	}
	for i, tt := range tests {
		copyIn := append([]fileSeg(nil), tt.in...)
		got := trimLeadingSegBytes(tt.in, tt.trim)
		if !reflect.DeepEqual(tt.in, copyIn) {
			t.Fatalf("%d. trimLeadingSegBytes modified its input", i)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%d. trim = %+v; want %+v", i, got, tt.want)
		}
	}
}

func TestGetNewSegments(t *testing.T) {
	type testCase struct {
		name       string
		lastSegs   []fileSeg
		serverSegs [][]LogSegmentJSON

		// prefixSum is the prefix sum to use if called.
		// If empty, prefixSum calls are errors.
		prefixSum string

		want      []fileSeg
		wantSplit bool
	}
	tests := []testCase{
		{
			name: "first_download",
			serverSegs: [][]LogSegmentJSON{
				[]LogSegmentJSON{
					{Number: 1, Size: 100, SHA224: "abc"},
					{Number: 2, Size: 200, SHA224: "def"},
				},
			},
			want: []fileSeg{
				{seg: 1, size: 100, sha224: "abc", file: "/fake/0001.mutlog"},
				{seg: 2, size: 200, sha224: "def", file: "/fake/0002.mutlog"},
			},
		},
		{
			name: "incremental_download_growseg", // from first_download, segment 2 grows a bit
			lastSegs: []fileSeg{
				{seg: 1, size: 100, sha224: "abc", file: "/fake/0001.mutlog"},
				{seg: 2, size: 200, sha224: "def", file: "/fake/0002.mutlog"},
			},
			prefixSum: "def",
			serverSegs: [][]LogSegmentJSON{
				[]LogSegmentJSON{
					{Number: 1, Size: 100, SHA224: "abc"},
					{Number: 2, Size: 205, SHA224: "defdef"},
				},
			},
			want: []fileSeg{
				{seg: 2, size: 205, sha224: "defdef", skip: 200, file: "/fake/0002.mutlog"},
			},
		},
		{
			name: "incremental_download_growseg_and_newseg", // from first_download, segment 2 grows, and segment 3 appears.
			lastSegs: []fileSeg{
				{seg: 1, size: 100, sha224: "abc", file: "/fake/0001.mutlog"},
				{seg: 2, size: 200, sha224: "def", file: "/fake/0002.mutlog"},
			},
			prefixSum: "def",
			serverSegs: [][]LogSegmentJSON{
				[]LogSegmentJSON{
					{Number: 1, Size: 100, SHA224: "abc"},
					{Number: 2, Size: 250, SHA224: "defdef"},
					{Number: 3, Size: 300, SHA224: "fff"},
				},
			},
			want: []fileSeg{
				{seg: 2, size: 250, sha224: "defdef", skip: 200, file: "/fake/0002.mutlog"},
				{seg: 3, size: 300, sha224: "fff", skip: 0, file: "/fake/0003.mutlog"},
			},
		},
		{
			name: "incremental_download_newseg", // from first_download, segment 3 appears.
			lastSegs: []fileSeg{
				{seg: 1, size: 100, sha224: "abc", file: "/fake/0001.mutlog"},
				{seg: 2, size: 200, sha224: "def", file: "/fake/0002.mutlog"},
			},
			serverSegs: [][]LogSegmentJSON{
				[]LogSegmentJSON{
					{Number: 1, Size: 100, SHA224: "abc"},
					{Number: 2, Size: 200, SHA224: "def"},
					{Number: 3, Size: 300, SHA224: "fff"},
				},
			},
			want: []fileSeg{
				{seg: 3, size: 300, sha224: "fff", skip: 0, file: "/fake/0003.mutlog"},
			},
		},
		{
			name: "incremental_with_sleep",
			lastSegs: []fileSeg{
				{seg: 1, size: 101, sha224: "abc", file: "/fake/0001.mutlog"},
			},
			serverSegs: [][]LogSegmentJSON{
				[]LogSegmentJSON{
					{Number: 1, Size: 101, SHA224: "abc"},
				},
				[]LogSegmentJSON{
					{Number: 1, Size: 101, SHA224: "abc"},
					{Number: 2, Size: 102, SHA224: "def"},
				},
			},
			want: []fileSeg{
				{seg: 2, size: 102, sha224: "def", skip: 0, file: "/fake/0002.mutlog"},
			},
		},
		{
			name: "split_error_diff_first_seg_same_size",
			lastSegs: []fileSeg{
				{seg: 1, size: 101, sha224: "abc", file: "/fake/0001.mutlog"},
			},
			serverSegs: [][]LogSegmentJSON{
				[]LogSegmentJSON{
					{Number: 1, Size: 101, SHA224: "def"},
				},
			},
			wantSplit: true,
		},
		{
			name: "split_error_diff_first_seg_and_longer",
			lastSegs: []fileSeg{
				{seg: 1, size: 101, sha224: "abc", file: "/fake/0001.mutlog"},
			},
			serverSegs: [][]LogSegmentJSON{
				[]LogSegmentJSON{
					{Number: 1, Size: 102, SHA224: "def"},
				},
			},
			prefixSum: "ffffffffff", // no match
			wantSplit: true,
		},
		{
			name: "split_error_diff_first_seg_and_shorter",
			lastSegs: []fileSeg{
				{seg: 1, size: 101, sha224: "abc", file: "/fake/0001.mutlog"},
			},
			serverSegs: [][]LogSegmentJSON{
				[]LogSegmentJSON{
					{Number: 1, Size: 50, SHA224: "def"},
				},
			},
			prefixSum: "ffffffffff", // no match
			wantSplit: true,
		},
		{
			name: "split_error_diff_final_seg",
			lastSegs: []fileSeg{
				{seg: 1, size: 100, sha224: "abc", file: "/fake/0001.mutlog"},
				{seg: 2, size: 2, sha224: "def", file: "/fake/0002.mutlog"},
			},
			serverSegs: [][]LogSegmentJSON{
				[]LogSegmentJSON{
					{Number: 1, Size: 100, SHA224: "abc"},
					{Number: 2, Size: 4, SHA224: "fff"},
				},
			},
			prefixSum: "not_def",
			wantSplit: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverSegCalls := 0
			waits := 0
			ns := &netMutSource{
				last: tt.lastSegs,
				testHookGetServerSegments: func(_ context.Context, waitSizeNot int64) (segs []LogSegmentJSON, err error) {
					serverSegCalls++
					if serverSegCalls > 10 {
						t.Fatalf("infinite loop calling getServerSegments? num wait calls = %v", waits)
					}
					if len(tt.serverSegs) == 0 {
						return nil, nil
					}
					segs = tt.serverSegs[0]
					if len(tt.serverSegs) > 1 {
						tt.serverSegs = tt.serverSegs[1:]
					}
					return segs, nil
				},
				testHookSyncSeg: func(_ context.Context, seg LogSegmentJSON) (fileSeg, error) {
					return fileSeg{
						seg:    seg.Number,
						size:   seg.Size,
						sha224: seg.SHA224,
						file:   fmt.Sprintf("/fake/%04d.mutlog", seg.Number),
					}, nil
				},
				testHookFilePrefixSum224: func(file string, n int64) string {
					if tt.prefixSum != "" {
						return tt.prefixSum
					}
					t.Errorf("unexpected call to filePrefixSum224(%q, %d)", file, n)
					return "XXXX"
				},
			}
			got, err := ns.getNewSegments(context.Background())
			if tt.wantSplit && err == ErrSplit {
				// Success.
				return
			}
			if tt.wantSplit {
				t.Fatalf("wanted ErrSplit; got %+v, %v", got, err)
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mismatch\n got: %+v\nwant: %+v\n", got, tt.want)
			}
		})
	}
}
