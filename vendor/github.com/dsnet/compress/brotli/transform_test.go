// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

import "testing"

func TestTransform(t *testing.T) {
	vectors := []struct {
		id     int
		input  string
		output string
	}{
		{id: 0, input: "Hello, world!", output: "Hello, world!"},
		{id: 23, input: "groups of", output: "groups"},
		{id: 42, input: "s for the ", output: "s for "},
		{id: 48, input: "presentation", output: "prese"},
		{id: 56, input: "maintenance", output: "maint"},
		{id: 23, input: "Alexandria", output: "Alexand"},
		{id: 23, input: "archives", output: "archi"},
		{id: 49, input: "fighty", output: "fighting "},
		{id: 49, input: "12", output: "1ing "},
		{id: 49, input: "1", output: "ing "},
		{id: 49, input: "", output: "ing "},
		{id: 64, input: "123456789a", output: "1"},
		{id: 64, input: "123456789", output: ""},
		{id: 64, input: "1", output: ""},
		{id: 64, input: "", output: ""},
		{id: 3, input: "afloat", output: "float"},
		{id: 3, input: "12", output: "2"},
		{id: 3, input: "1", output: ""},
		{id: 3, input: "", output: ""},
		{id: 54, input: "123456789a", output: "a"},
		{id: 54, input: "123456789", output: ""},
		{id: 54, input: "1", output: ""},
		{id: 54, input: "", output: ""},
		{id: 73, input: "", output: " the  of the "},
		{id: 73, input: "dichlorodifluoromethanes", output: " the dichlorodifluoromethanes of the "},
		{id: 15, input: "", output: "  "},
		{id: 15, input: "meow", output: " Meow "},
		{id: 15, input: "-scale", output: " -scale "},
		{id: 15, input: "почти", output: " Почти "},
		{id: 15, input: "互联网", output: " 亗联网 "},
		{id: 119, input: "", output: " ='"},
		{id: 119, input: "meow", output: " MEOW='"},
		{id: 119, input: "-scale", output: " -SCALE='"},
		{id: 119, input: "почти", output: " ПОѧѢИ='"},
		{id: 119, input: "互联网", output: " 亗聑罔='"},
	}

	var buf [maxWordSize]byte
	for i, v := range vectors {
		cnt := transformWord(buf[:], []byte(v.input), v.id)
		output := string(buf[:cnt])

		if output != v.output {
			t.Errorf("test %d, output mismatch: got %q, want %q", i, output, v.output)
		}
	}
}
