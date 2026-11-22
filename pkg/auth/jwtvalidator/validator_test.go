//go:build test_unit

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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/auth/configReloader"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

const (
	testKid = "key-1"

	// static timestamps: the far-future one never expires, the past one has always expired
	farFuture = int64(4102444800) // 2100-01-01
	farPast   = int64(946684800)  // 2000-01-01
)

// fakeKeyProvider serves keys from a map and counts the lookups. When err is set, every lookup fails with it.
type fakeKeyProvider struct {
	keys      map[string]*Key
	callCount int
	err       error
}

func (p *fakeKeyProvider) GetKey(_ context.Context, _ *configReloader.Config, kid string) (*Key, error) {
	p.callCount++
	if p.err != nil {
		return nil, p.err
	}
	key, found := p.keys[kid]
	if !found {
		return nil, errors.Wrap(ErrNoKey, "No key for kid")
	}
	return key, nil
}

type ValidatorTestSuite struct {
	suite.Suite
	rsaKey      *rsa.PrivateKey
	otherRSAKey *rsa.PrivateKey
	ecKey       *ecdsa.PrivateKey
	logger      logger.Logger
	provider    *fakeKeyProvider
	validator   *Validator
	issuer      *configReloader.Config
}

func (suite *ValidatorTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("validator-test")
	suite.Require().NoError(err)
	suite.rsaKey, err = rsa.GenerateKey(rand.Reader, 2048)
	suite.Require().NoError(err)
	suite.otherRSAKey, err = rsa.GenerateKey(rand.Reader, 2048)
	suite.Require().NoError(err)
	suite.ecKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	suite.Require().NoError(err)
}

func (suite *ValidatorTestSuite) SetupTest() {
	suite.provider = &fakeKeyProvider{keys: map[string]*Key{
		testKid:   {PublicKey: &suite.rsaKey.PublicKey},
		"ec-key":  {PublicKey: &suite.ecKey.PublicKey},
		"pinned":  {PublicKey: &suite.rsaKey.PublicKey, Algorithm: "RS256"},
		"pinned2": {PublicKey: &suite.rsaKey.PublicKey, Algorithm: "PS256"},
	}}
	suite.validator = NewValidator(suite.logger)
	suite.validator.keyProvider = suite.provider
	suite.issuer = &configReloader.Config{
		IssuerURL:     testIssuerURL,
		IdentityClaim: configReloader.DefaultIdentityClaim,
	}
}

func (suite *ValidatorTestSuite) TestValidTokens() {
	for _, testCase := range []struct {
		name  string
		token string
	}{
		{name: "RS256", token: suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, testKid, suite.validClaims())},
		{name: "PS256", token: suite.signToken(jwt.SigningMethodPS256, suite.rsaKey, testKid, suite.validClaims())},
		{name: "ES256", token: suite.signToken(jwt.SigningMethodES256, suite.ecKey, "ec-key", suite.validClaims())},
		{name: "key declares the same algorithm", token: suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, "pinned", suite.validClaims())},
	} {
		suite.Run(testCase.name, func() {
			identity, err := suite.validate("Bearer " + testCase.token)
			suite.Require().NoError(err)
			suite.Require().Equal("caller-1", identity)
		})
	}
}

func (suite *ValidatorTestSuite) TestBearerSchemeIsCaseInsensitive() {
	token := suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, testKid, suite.validClaims())

	identity, err := suite.validate("bEaReR " + token)
	suite.Require().NoError(err)
	suite.Require().Equal("caller-1", identity)
}

func (suite *ValidatorTestSuite) TestRejectedTokens() {
	validToken := suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, testKid, suite.validClaims())
	tamperedToken := suite.tamperPayload(validToken)

	for _, testCase := range []struct {
		name          string
		header        string
		expectedError error
		expectLookup  bool
	}{
		{name: "no header", header: "", expectedError: jwt.ErrTokenMalformed},
		{name: "not a bearer token", header: "Basic " + validToken, expectedError: jwt.ErrTokenMalformed},
		{name: "token with two parts", header: "Bearer a.b", expectedError: jwt.ErrTokenMalformed},
		{name: "token over 16 KiB", header: "Bearer " + strings.Repeat("a", maxTokenSize) + ".b.c", expectedError: jwt.ErrTokenMalformed},
		{
			name:          "signed with another key",
			header:        "Bearer " + suite.signToken(jwt.SigningMethodRS256, suite.otherRSAKey, testKid, suite.validClaims()),
			expectedError: jwt.ErrTokenSignatureInvalid,
			expectLookup:  true,
		},
		{name: "tampered payload", header: "Bearer " + tamperedToken, expectedError: jwt.ErrTokenSignatureInvalid, expectLookup: true},
		{
			name:          "unknown issuer",
			header:        "Bearer " + suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, testKid, suite.claimsWith("iss", "https://other.example.com")),
			expectedError: jwt.ErrTokenInvalidIssuer,
		},
		{
			name:          "algorithm none",
			header:        "Bearer " + suite.signNone(suite.validClaims()),
			expectedError: ErrAlgNotAllowed,
		},
		{
			name:          "HS256 signed with the public key bytes",
			header:        "Bearer " + suite.signToken(jwt.SigningMethodHS256, []byte("public-key-bytes"), testKid, suite.validClaims()),
			expectedError: ErrAlgNotAllowed,
		},
		{
			name:          "algorithm differs from the key's declared algorithm",
			header:        "Bearer " + suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, "pinned2", suite.validClaims()),
			expectedError: ErrAlgMismatch,
			expectLookup:  true,
		},
		{
			name:          "RSA key used with an EC algorithm",
			header:        "Bearer " + suite.signToken(jwt.SigningMethodES256, suite.ecKey, testKid, suite.validClaims()),
			expectedError: jwt.ErrTokenSignatureInvalid,
			expectLookup:  true,
		},
		{
			name:          "unknown kid",
			header:        "Bearer " + suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, "unknown", suite.validClaims()),
			expectedError: ErrNoKey,
			expectLookup:  true,
		},
		{
			name:          "no kid",
			header:        "Bearer " + suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, "", suite.validClaims()),
			expectedError: ErrKid,
		},
		{
			name:          "kid with pipes",
			header:        "Bearer " + suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, "a||b", suite.validClaims()),
			expectedError: ErrKid,
		},
		{
			name:          "kid with a space",
			header:        "Bearer " + suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, "a b", suite.validClaims()),
			expectedError: ErrKid,
		},
		{
			name:          "kid with a quote",
			header:        "Bearer " + suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, `a"b`, suite.validClaims()),
			expectedError: ErrKid,
		},
		{
			name:          "kid with a control character",
			header:        "Bearer " + suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, "a\nb", suite.validClaims()),
			expectedError: ErrKid,
		},
		{
			name:          "kid of 300 characters",
			header:        "Bearer " + suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, strings.Repeat("a", 300), suite.validClaims()),
			expectedError: ErrKid,
		},
		{
			name:          "no expiry",
			header:        "Bearer " + suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, testKid, suite.claimsWithout("exp")),
			expectedError: jwt.ErrTokenRequiredClaimMissing,
			expectLookup:  true,
		},
		{
			name:          "expired",
			header:        "Bearer " + suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, testKid, suite.claimsWith("exp", farPast)),
			expectedError: jwt.ErrTokenExpired,
			expectLookup:  true,
		},
		{
			name:          "not valid yet",
			header:        "Bearer " + suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, testKid, suite.claimsWith("nbf", farFuture)),
			expectedError: jwt.ErrTokenNotValidYet,
			expectLookup:  true,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.provider.callCount = 0

			identity, err := suite.validate(testCase.header)
			suite.Require().Error(err)
			suite.Require().Empty(identity)
			suite.Require().True(errors.Is(err, testCase.expectedError), errors.GetErrorStackString(err, -1))

			// a rejection before the key lookup must not reach the key provider
			if !testCase.expectLookup {
				suite.Require().Zero(suite.provider.callCount)
			}
		})
	}
}

// TestKeyLookupFailure verifies a key provider failure denies the request and stays matchable in the error chain.
func (suite *ValidatorTestSuite) TestKeyLookupFailure() {
	providerError := errors.New("Failed to send request")
	suite.provider.err = providerError
	token := suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, testKid, suite.validClaims())

	_, err := suite.validate("Bearer " + token)
	suite.Require().Error(err)
	suite.Require().Contains(errors.GetErrorStackString(err, -1), "Failed to get key")
	suite.Require().True(errors.Is(err, providerError), errors.GetErrorStackString(err, -1))
}

func (suite *ValidatorTestSuite) TestAudience() {
	for _, testCase := range []struct {
		name      string
		audiences []string
		tokenAud  interface{} // nil means the claim is missing
		expectOK  bool
	}{
		{name: "not configured, claim missing", audiences: nil, tokenAud: nil, expectOK: true},
		{name: "not configured, claim present", audiences: nil, tokenAud: "anything", expectOK: true},
		{name: "string matches", audiences: []string{"api", "second"}, tokenAud: "second", expectOK: true},
		{name: "array has a match", audiences: []string{"api"}, tokenAud: []string{"other", "api"}, expectOK: true},
		{name: "string mismatch", audiences: []string{"api"}, tokenAud: "other", expectOK: false},
		{name: "array mismatch", audiences: []string{"api"}, tokenAud: []string{"other", "more"}, expectOK: false},
		{name: "claim missing", audiences: []string{"api"}, tokenAud: nil, expectOK: false},
	} {
		suite.Run(testCase.name, func() {
			claims := suite.validClaims()
			if testCase.tokenAud != nil {
				claims["aud"] = testCase.tokenAud
			}
			suite.issuer.Audiences = testCase.audiences
			token := suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, testKid, claims)

			_, err := suite.validate("Bearer " + token)
			if testCase.expectOK {
				suite.Require().NoError(err)
				return
			}
			suite.Require().True(errors.Is(err, jwt.ErrTokenInvalidAudience), errors.GetErrorStackString(err, -1))
		})
	}
}

func (suite *ValidatorTestSuite) TestIdentity() {
	for _, testCase := range []struct {
		name          string
		identityClaim string
		claimValue    interface{} // nil means the claim is missing
		expected      string
		expectOK      bool
	}{
		{name: "default claim", identityClaim: "sub", claimValue: "caller-1", expected: "caller-1", expectOK: true},
		{name: "configured claim", identityClaim: "email", claimValue: "caller@example.com", expected: "caller@example.com", expectOK: true},
		{name: "missing", identityClaim: "email", claimValue: nil},
		{name: "empty string", identityClaim: "email", claimValue: ""},
		{name: "not a string", identityClaim: "email", claimValue: 42},
		{name: "carriage return and line feed", identityClaim: "email", claimValue: "caller\r\nX-Injected: 1"},
		{name: "only spaces", identityClaim: "email", claimValue: "   "},
		{name: "leading space", identityClaim: "email", claimValue: " caller"},
		{name: "trailing space", identityClaim: "email", claimValue: "caller "},
		{name: "too long", identityClaim: "email", claimValue: strings.Repeat("a", maxIdentitySize+1)},
	} {
		suite.Run(testCase.name, func() {
			claims := suite.validClaims()
			if testCase.claimValue != nil {
				claims[testCase.identityClaim] = testCase.claimValue
			}
			suite.issuer.IdentityClaim = testCase.identityClaim
			token := suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, testKid, claims)

			identity, err := suite.validate("Bearer " + token)
			if testCase.expectOK {
				suite.Require().NoError(err)
				suite.Require().Equal(testCase.expected, identity)
				return
			}
			suite.Require().True(errors.Is(err, ErrIdentity), errors.GetErrorStackString(err, -1))
			suite.Require().Empty(identity)
		})
	}
}

func (suite *ValidatorTestSuite) TestIssuerTrailingSlash() {
	for _, testCase := range []struct {
		name        string
		configured  string
		tokenIssuer string
	}{
		{name: "token issuer has a trailing slash", configured: testIssuerURL, tokenIssuer: testIssuerURL + "/"},
		{name: "configured issuer has a trailing slash", configured: testIssuerURL + "/", tokenIssuer: testIssuerURL},
		{name: "both have a trailing slash", configured: testIssuerURL + "/", tokenIssuer: testIssuerURL + "/"},
	} {
		suite.Run(testCase.name, func() {
			suite.issuer.IssuerURL = testCase.configured
			token := suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, testKid, suite.claimsWith("iss", testCase.tokenIssuer))

			identity, err := suite.validate("Bearer " + token)
			suite.Require().NoError(err)
			suite.Require().Equal("caller-1", identity)
		})
	}

	suite.Run("different issuer with a trailing slash", func() {
		token := suite.signToken(jwt.SigningMethodRS256, suite.rsaKey, testKid, suite.claimsWith("iss", "https://other.example.com/"))

		_, err := suite.validate("Bearer " + token)
		suite.Require().True(errors.Is(err, jwt.ErrTokenInvalidIssuer), errors.GetErrorStackString(err, -1))
	})
}

// --- test helpers (private, below public functions) ---

// validate validates the Authorization header and returns the identity, as the sidecar does.
func (suite *ValidatorTestSuite) validate(authorizationHeader string) (string, error) {
	return suite.validator.Validate(context.Background(), authorizationHeader, suite.issuer)
}

func (suite *ValidatorTestSuite) validClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"iss": testIssuerURL,
		"sub": "caller-1",
		"exp": farFuture,
	}
}

func (suite *ValidatorTestSuite) claimsWith(name string, value interface{}) jwt.MapClaims {
	claims := suite.validClaims()
	claims[name] = value
	return claims
}

func (suite *ValidatorTestSuite) claimsWithout(name string) jwt.MapClaims {
	claims := suite.validClaims()
	delete(claims, name)
	return claims
}

func (suite *ValidatorTestSuite) signToken(method jwt.SigningMethod, key interface{}, kid string, claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(method, claims)
	if kid != "" {
		token.Header["kid"] = kid
	}

	signed, err := token.SignedString(key)
	suite.Require().NoError(err)
	return signed
}

func (suite *ValidatorTestSuite) signNone(claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	token.Header["kid"] = testKid

	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	suite.Require().NoError(err)
	return signed
}

// tamperPayload replaces the payload of a signed token and keeps its signature.
func (suite *ValidatorTestSuite) tamperPayload(token string) string {
	parts := strings.Split(token, ".")
	suite.Require().Len(parts, 3)

	parts[1] = base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"` + testIssuerURL + `","sub":"attacker","exp":4102444800}`))
	return strings.Join(parts, ".")
}

func TestValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(ValidatorTestSuite))
}
