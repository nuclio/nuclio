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
	"sync"
	"time"

	"github.com/nuclio/nuclio/pkg/auth/configReloader"

	gojose "github.com/go-jose/go-jose/v4"
	"github.com/nuclio/errors"
)

const (
	maxTokenSize    = 16 * 1024
	maxIdentitySize = 1024
	leeway          = 30 * time.Second
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

// Errors for the checks the jwt package has no error for. The other checks return the jwt package's errors.
var (
	ErrAlgNotAllowed = errors.New("Algorithm not allowed")
	ErrAlgMismatch   = errors.New("Algorithm does not match key")
	ErrKid           = errors.New("Invalid key ID")
	ErrNoKey         = errors.New("No usable key")
	ErrIdentity      = errors.New("Invalid identity claim")
)

// Key is a public key of the identity provider.
type Key struct {
	PublicKey crypto.PublicKey

	// Algorithm is the algorithm the identity provider declares for the key, empty when it declares none.
	Algorithm string
}

// Equal reports whether other holds the same public key and declares the same algorithm.
func (k *Key) Equal(other *Key) bool {
	publicKey, ok := k.PublicKey.(interface{ Equal(crypto.PublicKey) bool })
	return ok && k.Algorithm == other.Algorithm && publicKey.Equal(other.PublicKey)
}

// KeyProvider is the key layer: it owns everything about getting the identity provider's keys (downloading
// them, sharing in-flight downloads and caching them later), so the Validator never does I/O.
//
// GetKey returns the usable key for kid, or an error wrapping ErrNoKey when the identity provider has no
// usable key for it. A usable key is a public signing key that passes the key policy, so callers do not
// re-check it. An implementation must be safe for concurrent use, and must stop when ctx is done.
//
// FlushKeys drops the keys it keeps. The Validator calls it when the issuer's key set location changed.
type KeyProvider interface {
	GetKey(ctx context.Context, issuer *configReloader.Config, kid string) (*Key, error)
	FlushKeys()
}

// CacheConfig are the platform-wide settings of the key caches
type CacheConfig struct {
	// KeyTTL is how long a downloaded key is used, unless the issuer entry sets its own TTL.
	KeyTTL time.Duration

	// RejectedKidTTL is how long a key ID with no usable key is rejected without a download.
	RejectedKidTTL time.Duration

	// RejectedKidCacheSize is the most key IDs the rejected key ID cache holds.
	RejectedKidCacheSize int

	// DownloadGap is the least time between two key set downloads.
	DownloadGap time.Duration

	// FailureBackoff is how long no download starts after a failed one.
	FailureBackoff time.Duration

	// MaxPendingRequests is the most requests held for a download at once, at least 1.
	MaxPendingRequests int
}

type openIDConfiguration struct {
	Issuer  string `json:"issuer"`
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

// keyEntry is a cached key, with when it was downloaded.
type keyEntry struct {
	key          *Key
	downloadedAt time.Time
}

// keyCache holds the downloaded keys by key ID.
type keyCache struct {
	sync.RWMutex
	entries map[string]*keyEntry
}

// rejectedKidCache holds the key IDs that recently had no usable key, with when each stops being rejected.
type rejectedKidCache struct {
	sync.RWMutex
	ttl       time.Duration
	size      int
	expiresAt map[string]time.Time
}
