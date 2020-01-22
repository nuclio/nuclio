// Copyright 2018 by David A. Golden. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package scram

import "testing"

func TestEncodeName(t *testing.T) {
	cases := []struct {
		input  string
		expect string
	}{
		{input: "arthur", expect: "arthur"},
		{input: "doe,jane", expect: "doe=2Cjane"},
		{input: "a,b,c,d", expect: "a=2Cb=2Cc=2Cd"},
		{input: "a,b=c,d=", expect: "a=2Cb=3Dc=2Cd=3D"},
	}

	for _, c := range cases {
		if got := encodeName(c.input); got != c.expect {
			t.Errorf("Failed encoding '%s', got '%s', expected '%s'", c.input, got, c.expect)
		}
	}
}
