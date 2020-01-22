// Copyright 2018 by David A. Golden. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package scram

import (
	"strconv"
	"testing"
)

func TestClientConv(t *testing.T) {
	cases, err := getTestData("good", "bad-server")
	if err != nil {
		t.Fatal(err)
	}

	for _, v := range cases {
		t.Run(v.Label, genClientSubTest(v))
	}
}

func genClientSubTest(c TestCase) func(t *testing.T) {
	return func(t *testing.T) {
		hgf, err := getHGF(c.Digest)
		if err != nil {
			t.Fatal(err)
		}

		var client *Client
		if c.SkipSASLprep {
			client, err = hgf.NewClientUnprepped(c.User, c.Pass, c.AuthzID)
		} else {
			client, err = hgf.NewClient(c.User, c.Pass, c.AuthzID)
		}
		if err != nil {
			t.Errorf("%s: expected no error from NewClient, but got '%v'", c.Label, err)
		}
		if c.ClientNonce != "" {
			client = client.WithNonceGenerator(func() string { return c.ClientNonce })
		}
		conv := client.NewConversation()

		for i, s := range clientSteps(c) {
			if conv.Done() {
				t.Errorf("%s: Premature end of conversation before step %d", c.Label, i+1)
				return
			}
			got, err := conv.Step(s.Input)
			if s.IsError && err == nil {
				t.Errorf("%s: step %d: expected error but didn't get one", c.Label, i+1)
				return
			} else if !s.IsError && err != nil {
				t.Errorf("%s: step %d: expected no error but got '%v'", c.Label, i+1, err)
				return
			}
			if got != s.Expect {
				t.Errorf("%s: step %d: incorrect step message; got %s, expected %s",
					c.Label,
					i+1,
					strconv.QuoteToASCII(got),
					strconv.QuoteToASCII(s.Expect),
				)
				return
			}
		}

		if c.Valid != conv.Valid() {
			t.Errorf("%s: Conversation Valid() incorrect: got '%v', expected '%v'", c.Label, conv.Valid(), c.Valid)
			return
		}

		if !conv.Done() {
			t.Errorf("%s: Conversation not marked done after last step", c.Label)
		}
	}
}
