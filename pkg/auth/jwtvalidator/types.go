/*
Copyright 2026 The Nuclio Authors.

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

package jwtvalidator

import (
	"context"
	"crypto"
	"encoding/json"
	"regexp"

	"github.com/nuclio/nuclio/pkg/auth/configReloader"

	gojose "github.com/go-jose/go-jose/v4"
	"github.com/nuclio/errors"
)

// allowedAlgorithms are the only signing algorithms accepted, regardless of what the token or the key says.
var allowedAlgorithms = map[string]struct{}{
	string(gojose.RS256): {},
	string(gojose.ES256): {},
	string(gojose.PS256): {},
}

func isAllowedAlgorithm(algorithm string) bool {
	_, allowed := allowedAlgorithms[algorithm]
	return allowed
}

var kidPattern = regexp.MustCompile(`^[A-Za-z0-9._~:+/=-]{1,256}$`)

// ErrNoKey is returned when the identity provider has no usable key for a key ID.
var ErrNoKey = errors.New("No usable key")

// Key is a public key of the identity provider.
type Key struct {
	PublicKey crypto.PublicKey

	// Algorithm is the algorithm the identity provider declares for the key, empty when it declares none.
	Algorithm string
}

// KeyProvider is the key layer: it owns everything about getting the identity provider's keys (downloading
// them now, sharing in-flight downloads and caching them later), so the Validator never does I/O.
//
// GetKey returns the usable key for kid, or an error wrapping ErrNoKey when the identity provider has no
// usable key for it. A usable key is a public signing key that passes the key policy, so callers do not
// re-check it. An implementation must be safe for concurrent use, and must stop when ctx is done.
// issuer is passed on every call, so an implementation that keeps keys can tell when the issuer changed.
type KeyProvider interface {
	GetKey(ctx context.Context, issuer *configReloader.Config, kid string) (*Key, error)
}

type openIDConfiguration struct {
	JwksURI string `json:"jwks_uri"`
}

// keySet keeps each key raw, so a key that fails to parse does not fail the whole set.
type keySet struct {
	Keys []json.RawMessage `json:"keys"`
}

// keyOperations reads the key_ops field, which go-jose does not expose.
type keyOperations struct {
	KeyOps []string `json:"key_ops"`
}
