// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gerrit

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func md5str(text string) string {
	h := md5.Sum([]byte(text))
	return hex.EncodeToString(h[:])
}

func TestBasicAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "User Password true"
		u, p, ok := r.BasicAuth()
		if expected != fmt.Sprintf("%s %s %t", u, p, ok) {
			t.Errorf("Expected %s, got %s %s %t", expected, u, p, ok)
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			// The JSON response begins with an XSRF-defeating header ")]}\n"
			fmt.Fprintln(w, ")]}")
			json.NewEncoder(w).Encode(AccountInfo{})
		}
	}))
	defer ts.Close()

	_, err := NewClient(
		ts.URL,
		BasicAuth("User", "Password"),
	).GetAccountInfo(context.Background(), "self")
	if err != nil {
		t.Error(err)
	}
}

func TestDigestAuth(t *testing.T) {
	const (
		user   = "User"
		pass   = "Password"
		nonce  = "dcd98b7102dd2f0e8b11d0f600bfb0c093"
		opaque = "5ccc069c403ebaf9f0171e9517f40e41"
		realm  = "Gerrit Code Review"
		qop    = "auth"
	)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(
				`Digest realm="%s", qop="%s", nonce="%s", opaque="%s"`,
				realm, qop, nonce, opaque,
			))
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			parts := strings.SplitN(header, " ", 2)
			parts = strings.Split(parts[1], ", ")
			opts := make(map[string]string)

			for _, part := range parts {
				vals := strings.SplitN(part, "=", 2)
				key := vals[0]
				val := strings.Trim(vals[1], "\",")
				opts[key] = val
			}

			// https://en.wikipedia.org/wiki/Digest_access_authentication#Example_with_explanation
			// The "response" value is calculated in three steps, as follows.
			//   Where values are combined, they are delimited by colons.
			// 1. The MD5 hash of the combined username, authentication realm and password is calculated.
			//    The result is referred to as HA1.
			// 2. The MD5 hash of the combined method and digest URI is calculated, e.g. of "GET" and "/index.html".
			//    The result is referred to as HA2.
			// 3. The MD5 hash of the combined HA1 result, server nonce (nonce), request counter (nc),
			//    client nonce (cnonce), quality of protection code (qop) and HA2 result is calculated.
			//    The result is the "response" value provided by the client.
			ha1 := md5str(fmt.Sprintf("%s:%s:%s", user, realm, pass))
			ha2 := md5str("GET:/a/accounts/self")
			expected := md5str(fmt.Sprintf("%s:%s:%s:%s:%s:%s", ha1, nonce, opts["nc"], opts["cnonce"], qop, ha2))

			if expected != opts["response"] {
				t.Errorf("Expected %s, got %s", expected, opts["response"])
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				// The JSON response begins with an XSRF-defeating header ")]}\n"
				fmt.Fprintln(w, ")]}")
				json.NewEncoder(w).Encode(AccountInfo{})
			}
		}
	}))
	defer ts.Close()

	_, err := NewClient(
		ts.URL,
		DigestAuth(user, pass),
	).GetAccountInfo(context.Background(), "self")
	if err != nil {
		t.Error(err)
	}
}
