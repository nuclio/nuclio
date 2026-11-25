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

// Package jwtvalidator verifies JWTs issued by an external identity provider against its public keys.
// It knows nothing about files, Kubernetes or functions: it gets a token and an issuer entry.
//
// The package has two layers:
//
//	Validator                the token rules only: header, algorithm, kid, issuer, signature, expiry,
//	  │                      audience. It asks the KeyProvider for the key and does no other I/O
//	  └─ KeyProvider         the key layer, an interface: downloads the identity provider's keys and
//	                         shares in-flight downloads (JWKSDownloader), and later caches them
//
// Putting the keys behind an interface lets a different deployment (for example the DLX) use the same
// Validator with a different KeyProvider, such as one with no cache.
package jwtvalidator

import (
	"context"
	"slices"
	"strings"
	"sync/atomic"
	"unicode"

	"github.com/nuclio/nuclio/pkg/auth/configReloader"
	"github.com/nuclio/nuclio/pkg/auth/utils"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

// Validator checks the rules a token must meet, taking the identity provider's keys from a KeyProvider.
type Validator struct {
	logger      logger.Logger
	keyProvider KeyProvider

	// lastIssuer is the issuer entry the keys were last used with. The reloader publishes a new entry only
	// when it really changed.
	lastIssuer atomic.Pointer[configReloader.Config]
}

// NewValidator creates a validator that downloads and caches the identity provider's keys.
func NewValidator(parentLogger logger.Logger, cacheConfig *CacheConfig) *Validator {
	return &Validator{
		logger:      parentLogger.GetChild("jwt-validator"),
		keyProvider: NewJWKSDownloader(parentLogger, cacheConfig),
	}
}

// Validate checks an Authorization header value ("Bearer <token>") against the issuer entry and returns the
// caller's identity. Every check must pass, and any error means the request must be denied.
func (v *Validator) Validate(ctx context.Context, authorizationHeader string, issuer *configReloader.Config) (string, error) {
	v.flushKeysOnIssuerChange(ctx, issuer)

	token, err := v.extractJWT(authorizationHeader)
	if err != nil {
		return "", errors.Wrap(err, "Failed to extract bearer token")
	}

	// the key is looked up while the token is parsed, but only after its algorithm, kid and issuer passed
	claims := jwt.MapClaims{}
	parser := jwt.NewParser(jwt.WithLeeway(leeway), jwt.WithExpirationRequired())

	if _, err := parser.ParseWithClaims(token, claims, func(parsedToken *jwt.Token) (interface{}, error) {
		key, err := v.resolveKey(ctx, parsedToken, issuer)
		// log the error to have a detailed stack trace
		if err != nil {
			v.logger.WarnWithCtx(ctx, "Failed to resolve key", "err", errors.GetErrorStackString(err, -1))
		}
		return key, err
	}); err != nil {
		return "", errors.Wrap(err, "Failed to verify token")
	}

	if err := v.checkAudience(claims, issuer.Audiences); err != nil {
		return "", errors.Wrap(err, "Failed to check token audience")
	}

	identity, err := v.identityFromClaims(claims, issuer.IdentityClaim)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get identity from token")
	}
	return identity, nil
}

// identityFromClaims returns the caller's identity from the claims of a verified token.
func (v *Validator) identityFromClaims(claims jwt.MapClaims, identityClaim string) (string, error) {
	identity, ok := claims[identityClaim].(string)
	if !ok || identity == "" {
		return "", errors.Wrapf(ErrIdentity, "Identity claim is missing, empty or not a string (claim=%s)", identityClaim)
	}

	// the HTTP transport trims surrounding whitespace from header values, so the forwarded identity would differ from the claim
	if identity != strings.TrimSpace(identity) {
		return "", errors.Wrapf(ErrIdentity, "Identity claim has surrounding whitespace (claim=%s)", identityClaim)
	}

	if len(identity) > maxIdentitySize {
		return "", errors.Wrapf(ErrIdentity, "Identity claim is too long (claim=%s)", identityClaim)
	}

	// the identity is forwarded in a request header
	if strings.ContainsFunc(identity, unicode.IsControl) {
		return "", errors.Wrapf(ErrIdentity, "Identity claim has control characters (claim=%s)", identityClaim)
	}
	return identity, nil
}

// flushKeysOnIssuerChange drops the cached keys when the issuer entry points at a different key set. Only
// the request that swaps the entry in does it. The first entry has nothing to flush.
func (v *Validator) flushKeysOnIssuerChange(ctx context.Context, issuer *configReloader.Config) {
	if issuer == nil {
		v.logger.WarnWithCtx(ctx, "Issuer entry is missing, skipping key flush")
		return
	}

	// the reloader publishes a new entry only on a real change, so the same pointer means no change; when
	// another request swapped the entry in first, it does the flush
	previous := v.lastIssuer.Load()
	if previous == issuer || !v.lastIssuer.CompareAndSwap(previous, issuer) {
		return
	}

	// no previous entry means this is the first request after the start, when the cache is empty anyway
	if previous != nil && previous.JwksURL != issuer.JwksURL {
		v.logger.InfoWithCtx(ctx, "Key set URL changed, flushing cached keys", "jwksURL", issuer.JwksURL)
		v.keyProvider.FlushKeys()
	}
}

// resolveKey runs the checks that need only the unverified token, then returns the key to verify it with.
func (v *Validator) resolveKey(ctx context.Context, token *jwt.Token, issuer *configReloader.Config) (interface{}, error) {
	algorithm := token.Method.Alg()
	if !isAllowedAlgorithm(algorithm) {
		return nil, errors.Wrapf(ErrAlgNotAllowed, "Token algorithm is not allowed (alg=%s)", algorithm)
	}

	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, errors.Wrap(ErrKid, "Token key ID is missing or not a string")
	}
	if !kidPattern.MatchString(kid) {
		return nil, errors.Wrap(ErrKid, "Token key ID has invalid characters or length")
	}

	tokenIssuer, err := token.Claims.GetIssuer()
	if err != nil {
		return nil, errors.Wrap(jwt.ErrTokenInvalidIssuer, "Token issuer is malformed")
	}
	if strings.TrimRight(tokenIssuer, "/") != strings.TrimRight(issuer.IssuerURL, "/") {
		return nil, errors.Wrap(jwt.ErrTokenInvalidIssuer, "Token issuer does not match the trusted issuer")
	}

	key, err := v.keyProvider.GetKey(ctx, issuer, kid)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get key (kid=%s)", kid)
	}
	if key.Algorithm != "" && key.Algorithm != algorithm {
		return nil, errors.Wrapf(ErrAlgMismatch, "Token algorithm differs from the key algorithm (alg=%s, keyAlg=%s)", algorithm, key.Algorithm)
	}
	return key.PublicKey, nil
}

func (v *Validator) extractJWT(authorizationHeader string) (string, error) {
	token, err := utils.ExtractBearerToken(authorizationHeader)
	if err != nil {
		return "", errors.Wrap(jwt.ErrTokenMalformed, err.Error())
	}

	if len(token) > maxTokenSize {
		return "", errors.Wrap(jwt.ErrTokenMalformed, "Token is too large")
	}

	if strings.Count(token, ".") != 2 {
		return "", errors.Wrap(jwt.ErrTokenMalformed, "Token does not have three parts")
	}
	return token, nil
}

// checkAudience passes when no audiences are configured, or when any token audience is configured.
func (v *Validator) checkAudience(claims jwt.MapClaims, audiences []string) error {
	if len(audiences) == 0 {
		return nil
	}

	tokenAudiences, err := claims.GetAudience()
	if err != nil {
		return errors.Wrap(jwt.ErrTokenInvalidAudience, "Token audience is malformed")
	}

	for _, tokenAudience := range tokenAudiences {
		if slices.Contains(audiences, tokenAudience) {
			return nil
		}
	}
	return errors.Wrap(jwt.ErrTokenInvalidAudience, "Token audience is not trusted")
}
