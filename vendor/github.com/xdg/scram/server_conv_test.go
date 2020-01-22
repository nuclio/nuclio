// Copyright 2018 by David A. Golden. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package scram

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"testing"

	"github.com/xdg/stringprep"
)

func TestServerConv(t *testing.T) {
	cases, err := getTestData("good", "bad-client")
	if err != nil {
		t.Fatal(err)
	}

	for _, v := range cases {
		t.Run(v.Label, genServerSubTest(v))
	}
}

// Prep user credential callback for the case from Client
func genServerCallback(c TestCase) (CredentialLookup, error) {
	salt, err := base64.StdEncoding.DecodeString(c.Salt64)
	if err != nil {
		return nil, fmt.Errorf("error decoding salt: %v", err)
	}

	hgf, err := getHGF(c.Digest)
	if err != nil {
		return nil, fmt.Errorf("error getting digest for credential callback: %v", err)
	}

	kf := KeyFactors{Salt: string(salt), Iters: c.Iters}

	var client *Client
	var userprep string
	if c.SkipSASLprep {
		client, err = hgf.NewClientUnprepped(c.User, c.Pass, c.AuthzID)
		userprep = c.User
	} else {
		client, err = hgf.NewClient(c.User, c.Pass, c.AuthzID)
		if userprep, err = stringprep.SASLprep.Prepare(c.User); err != nil {
			return nil, fmt.Errorf("Error SASLprepping username '%s': %v", c.User, err)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("error generating client for credential callback: %v", err)
	}

	stored := client.GetStoredCredentials(kf)

	cbFcn := func(s string) (StoredCredentials, error) {
		if s == userprep {
			return stored, nil
		}
		return StoredCredentials{}, fmt.Errorf("Unknown user %s", s)
	}

	return cbFcn, nil
}

func genServerSubTest(c TestCase) func(t *testing.T) {
	return func(t *testing.T) {
		hgf, err := getHGF(c.Digest)
		if err != nil {
			t.Fatal(err)
		}

		cbFcn, err := genServerCallback(c)
		if err != nil {
			t.Fatal(err)
		}

		server, err := hgf.NewServer(cbFcn)
		if err != nil {
			t.Fatalf("%s: expected no error from NewServer, but got '%v'", c.Label, err)
		}
		if c.ServerNonce != "" {
			server = server.WithNonceGenerator(func() string { return c.ServerNonce })
		}
		conv := server.NewConversation()

		for i, s := range serverSteps(c) {
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

		var expectedUser string
		if c.SkipSASLprep {
			expectedUser = c.User
		} else {
			if expectedUser, err = stringprep.SASLprep.Prepare(c.User); err != nil {
				t.Errorf("Error SASLprepping username '%s': %v", c.User, err)
			}
		}

		if conv.Valid() && conv.Username() != expectedUser {
			t.Errorf("%s: Conversation didn't record proper username: got '%s', expected '%s'", c.Label, conv.Username(), expectedUser)
		}
	}
}
