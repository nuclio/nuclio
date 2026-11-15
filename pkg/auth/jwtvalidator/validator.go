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
	"strings"
	"unicode"

	"github.com/nuclio/nuclio/pkg/auth/configReloader"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

// Validator checks the rules a token must meet, taking the identity provider's keys from a KeyProvider.
type Validator struct {
	keyProvider KeyProvider
}

// NewValidator creates a validator that downloads the identity provider's keys.
func NewValidator(parentLogger logger.Logger) *Validator {
	return &Validator{keyProvider: NewJWKSDownloader(parentLogger)}
}

// Validate checks an Authorization header value ("Bearer <token>") against the issuer entry. Every check
// must pass, and any error means the request must be denied.
func (v *Validator) Validate(ctx context.Context, authorizationHeader string, issuer *configReloader.Config) error {
	token, err := v.extractBearerToken(authorizationHeader)
	if err != nil {
		return errors.Wrap(err, "Failed to extract bearer token")
	}

	// the key is looked up while the token is parsed, but only after its algorithm, kid and issuer passed
	claims := jwt.MapClaims{}
	parser := jwt.NewParser(jwt.WithLeeway(leeway), jwt.WithExpirationRequired())

	// the parser keeps only the text of the key lookup error, so it is kept here to log the whole cause chain
	var keyError error
	if _, err := parser.ParseWithClaims(token, claims, func(parsedToken *jwt.Token) (interface{}, error) {
		key, err := v.resolveKey(ctx, parsedToken, issuer)
		keyError = err
		return key, err
	}); err != nil {
		if keyError != nil {
			err = keyError
		}
		return errors.Wrap(err, "Failed to verify token")
	}

	if err := v.checkAudience(claims, issuer.Audiences); err != nil {
		return errors.Wrap(err, "Failed to check token audience")
	}
	return nil
}

// ExtractIdentity returns the caller's identity from an Authorization header value. It does not verify the
// token, so it must only be called with a header that Validate accepted.
func (v *Validator) ExtractIdentity(authorizationHeader string, identityClaim string) (string, error) {
	token, err := v.extractBearerToken(authorizationHeader)
	if err != nil {
		return "", errors.Wrap(err, "Failed to extract bearer token")
	}

	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(token, claims); err != nil {
		return "", errors.Wrap(jwt.ErrTokenMalformed, "Failed to parse token claims")
	}

	identity, ok := claims[identityClaim].(string)
	if !ok || identity == "" {
		return "", errors.Wrapf(ErrIdentity, "Identity claim is missing, empty or not a string (claim=%s)", identityClaim)
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
	if tokenIssuer != issuer.IssuerURL {
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

func (v *Validator) extractBearerToken(authorizationHeader string) (string, error) {
	scheme, token, found := strings.Cut(authorizationHeader, " ")
	if !found || !strings.EqualFold(scheme, bearerScheme) {
		return "", errors.Wrap(jwt.ErrTokenMalformed, "Authorization header is not a bearer token")
	}

	tokenLen := len(token)
	if tokenLen == 0 {
		return "", errors.Wrap(jwt.ErrTokenMalformed, "Token is empty")
	}

	if tokenLen > maxTokenSize {
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
		for _, audience := range audiences {
			if tokenAudience == audience {
				return nil
			}
		}
	}
	return errors.Wrap(jwt.ErrTokenInvalidAudience, "Token audience is not trusted")
}
