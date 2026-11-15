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
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/auth/configReloader"

	gojose "github.com/go-jose/go-jose/v4"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

const (
	testIssuerURL    = "https://idp.example.com/realms/test"
	testDiscoveryURL = "https://idp.example.com/realms/test/.well-known/openid-configuration"
	testJWKSURL      = "https://idp.example.com/realms/test/protocol/openid-connect/certs"
)

// extraFields are the web key fields added on top of the key material.
type extraFields map[string]interface{}

// fakeTransport answers requests from a map of URL to response and records the requested URLs.
type fakeTransport struct {
	responses map[string]fakeResponse
	requests  []string
}

type fakeResponse struct {
	status int
	body   []byte
}

func (t *fakeTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	t.requests = append(t.requests, request.URL.String())

	response, found := t.responses[request.URL.String()]
	if !found {
		return nil, errors.New("Unexpected request")
	}
	return &http.Response{
		StatusCode: response.status,
		Body:       io.NopCloser(bytes.NewReader(response.body)),
		Header:     http.Header{},
		Request:    request,
	}, nil
}

type JWKSDownloaderTestSuite struct {
	suite.Suite
	logger        logger.Logger
	rsaKey        *rsa.PrivateKey
	weakRSAKey    *rsa.PrivateKey
	ecKey         *ecdsa.PrivateKey
	transport     *fakeTransport
	downloader    *JWKSDownloader
	issuer        *configReloader.Config
	issuerWithURL *configReloader.Config
}

func (suite *JWKSDownloaderTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("jwks-downloader-test")
	suite.Require().NoError(err)

	suite.rsaKey, err = rsa.GenerateKey(rand.Reader, 2048)
	suite.Require().NoError(err)
	suite.weakRSAKey, err = rsa.GenerateKey(rand.Reader, 1024)
	suite.Require().NoError(err)
	suite.ecKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	suite.Require().NoError(err)
}

func (suite *JWKSDownloaderTestSuite) SetupTest() {
	suite.transport = &fakeTransport{responses: map[string]fakeResponse{}}
	suite.downloader = NewJWKSDownloader(suite.logger)
	suite.downloader.httpClient = &http.Client{Transport: suite.transport}
	suite.issuer = &configReloader.Config{IssuerURL: testIssuerURL}
	suite.issuerWithURL = &configReloader.Config{IssuerURL: testIssuerURL, JwksURL: testJWKSURL}
}

func (suite *JWKSDownloaderTestSuite) TestDiscoversKeySetAndReturnsKeys() {
	suite.serveDiscovery(testJWKSURL)
	suite.serveKeys(
		suite.webKey("rsa-key", &suite.rsaKey.PublicKey, extraFields{"alg": "RS256"}),
		suite.webKey("ec-key", &suite.ecKey.PublicKey, extraFields{"use": "sig"}),
	)

	for _, testCase := range []struct {
		name              string
		kid               string
		expectedAlgorithm string
	}{
		{name: "RSA key", kid: "rsa-key", expectedAlgorithm: "RS256"},
		{name: "EC key", kid: "ec-key", expectedAlgorithm: ""},
	} {
		suite.Run(testCase.name, func() {
			key, err := suite.downloader.GetKey(context.Background(), suite.issuer, testCase.kid)
			suite.Require().NoError(err)
			suite.Require().Equal(testCase.expectedAlgorithm, key.Algorithm)
		})
	}
}

func (suite *JWKSDownloaderTestSuite) TestDiscoveryUrlIgnoresTrailingSlash() {
	suite.serveDiscovery(testJWKSURL)
	suite.serveKeys(suite.webKey("rsa-key", &suite.rsaKey.PublicKey, nil))

	suite.issuer.IssuerURL += "/"
	_, err := suite.downloader.GetKey(context.Background(), suite.issuer, "rsa-key")
	suite.Require().NoError(err)
	suite.Require().Equal([]string{testDiscoveryURL, testJWKSURL}, suite.transport.requests)
}

func (suite *JWKSDownloaderTestSuite) TestConfiguredKeySetUrlSkipsDiscovery() {
	suite.serveKeys(suite.webKey("rsa-key", &suite.rsaKey.PublicKey, nil))

	_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "rsa-key")
	suite.Require().NoError(err)
	suite.Require().Equal([]string{testJWKSURL}, suite.transport.requests)
}

func (suite *JWKSDownloaderTestSuite) TestUnusableKeysAreSkipped() {
	suite.serveKeys(
		suite.webKey("good", &suite.rsaKey.PublicKey, nil),
		suite.webKey("encryption", &suite.rsaKey.PublicKey, extraFields{"use": "enc"}),
		suite.webKey("no-verify", &suite.rsaKey.PublicKey, extraFields{"key_ops": []string{"encrypt"}}),
		suite.webKey("weak", &suite.weakRSAKey.PublicKey, nil),
		suite.webKey("bad-alg", &suite.rsaKey.PublicKey, extraFields{"alg": "HS256"}),
		suite.webKey("wrong-type-alg", &suite.rsaKey.PublicKey, extraFields{"alg": "ES256"}),
		suite.webKey("ec-wrong-alg", &suite.ecKey.PublicKey, extraFields{"alg": "RS256"}),
		suite.webKey("bad kid", &suite.rsaKey.PublicKey, nil),
		suite.webKey("private", suite.rsaKey, nil),
		suite.webKey("symmetric", []byte("shared-secret"), nil),
		json.RawMessage(`{"kty":"RSA","kid":"broken","n":"!!","e":"AQAB"}`),
	)

	for _, testCase := range []struct {
		name     string
		kid      string
		expectOK bool
	}{
		{name: "usable key", kid: "good", expectOK: true},
		{name: "use is enc", kid: "encryption"},
		{name: "key_ops lacks verify", kid: "no-verify"},
		{name: "1024-bit RSA", kid: "weak"},
		{name: "algorithm not allowed", kid: "bad-alg"},
		{name: "algorithm does not fit an RSA key", kid: "wrong-type-alg"},
		{name: "algorithm does not fit an EC key", kid: "ec-wrong-alg"},
		{name: "key ID with invalid characters", kid: "bad kid"},
		{name: "private key", kid: "private"},
		{name: "symmetric key", kid: "symmetric"},
		{name: "key that fails to parse", kid: "broken"},
		{name: "kid not in the set", kid: "missing"},
	} {
		suite.Run(testCase.name, func() {
			_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, testCase.kid)
			if testCase.expectOK {
				suite.Require().NoError(err)
				return
			}
			suite.Require().True(errors.Is(err, ErrNoKey), errors.GetErrorStackString(err, -1))
		})
	}
}

func (suite *JWKSDownloaderTestSuite) TestKeySetWithOnlyUnusableKeysIsAnError() {
	suite.serveKeys(suite.webKey("weak", &suite.weakRSAKey.PublicKey, nil))

	_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "weak")
	suite.Require().True(errors.Is(err, ErrNoKey), errors.GetErrorStackString(err, -1))
}

func (suite *JWKSDownloaderTestSuite) TestParseUsableKeys() {
	otherCurveKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	suite.Require().NoError(err)

	for _, testCase := range []struct {
		name               string
		rawKeys            []json.RawMessage
		expectedAlgorithms map[string]string // usable key ID -> algorithm the key declares
	}{
		{
			name:               "no keys",
			rawKeys:            nil,
			expectedAlgorithms: map[string]string{},
		},
		{
			name: "RSA and EC keys, with and without a declared algorithm",
			rawKeys: []json.RawMessage{
				suite.webKey("rsa", &suite.rsaKey.PublicKey, nil),
				suite.webKey("rsa-rs256", &suite.rsaKey.PublicKey, extraFields{"alg": "RS256"}),
				suite.webKey("rsa-ps256", &suite.rsaKey.PublicKey, extraFields{"alg": "PS256"}),
				suite.webKey("ec", &suite.ecKey.PublicKey, nil),
				suite.webKey("ec-es256", &suite.ecKey.PublicKey, extraFields{"alg": "ES256"}),
			},
			expectedAlgorithms: map[string]string{
				"rsa":       "",
				"rsa-rs256": "RS256",
				"rsa-ps256": "PS256",
				"ec":        "",
				"ec-es256":  "ES256",
			},
		},
		{
			name: "key_ops that lists verify among other operations",
			rawKeys: []json.RawMessage{
				suite.webKey("sign-and-verify", &suite.rsaKey.PublicKey, extraFields{"key_ops": []string{"sign", "verify"}}),
			},
			expectedAlgorithms: map[string]string{"sign-and-verify": ""},
		},
		{
			name: "key_ops that is not a list",
			rawKeys: []json.RawMessage{
				suite.webKey("ops-string", &suite.rsaKey.PublicKey, extraFields{"key_ops": "verify"}),
			},
			expectedAlgorithms: map[string]string{},
		},
		{
			name: "EC key on a curve other than P-256",
			rawKeys: []json.RawMessage{
				suite.webKey("p384", &otherCurveKey.PublicKey, nil),
			},
			expectedAlgorithms: map[string]string{},
		},
		{
			name: "an unusable key does not hide the usable ones",
			rawKeys: []json.RawMessage{
				suite.webKey("weak", &suite.weakRSAKey.PublicKey, nil),
				suite.webKey("first", &suite.rsaKey.PublicKey, nil),
				json.RawMessage(`"not a key"`),
				json.RawMessage(`{"kty":"RSA","kid":"broken","n":"!!","e":"AQAB"}`),
				suite.webKey("second", &suite.ecKey.PublicKey, nil),
			},
			expectedAlgorithms: map[string]string{"first": "", "second": ""},
		},
	} {
		suite.Run(testCase.name, func() {
			usableKeys := parseUsableKeys(testCase.rawKeys)

			algorithms := map[string]string{}
			for keyID, key := range usableKeys {
				algorithms[keyID] = key.Algorithm
			}
			suite.Require().Equal(testCase.expectedAlgorithms, algorithms)
		})
	}
}

func (suite *JWKSDownloaderTestSuite) TestDownloadIsRejected() {
	oversizedBody := []byte(`{"keys":[],"padding":"` + strings.Repeat("a", maxDownloadBody) + `"}`)

	for _, testCase := range []struct {
		name             string
		issuer           *configReloader.Config
		responses        map[string]fakeResponse
		expectedRequests int
	}{
		{
			name:             "issuer URL is not https",
			issuer:           &configReloader.Config{IssuerURL: "http://idp.example.com/realms/test"},
			expectedRequests: 0,
		},
		{
			name:             "key set URL is not https",
			issuer:           &configReloader.Config{IssuerURL: testIssuerURL, JwksURL: "http://idp.example.com/certs"},
			expectedRequests: 0,
		},
		{
			name:   "discovery document points to a URL that is not https",
			issuer: suite.issuer,
			responses: map[string]fakeResponse{
				testDiscoveryURL: {status: http.StatusOK, body: []byte(`{"jwks_uri":"http://idp.example.com/certs"}`)},
			},
			expectedRequests: 1,
		},
		{
			name:   "key set body over 1 MiB",
			issuer: suite.issuerWithURL,
			responses: map[string]fakeResponse{
				testJWKSURL: {status: http.StatusOK, body: oversizedBody},
			},
			expectedRequests: 1,
		},
		{
			name:   "key set answers with an error status",
			issuer: suite.issuerWithURL,
			responses: map[string]fakeResponse{
				testJWKSURL: {status: http.StatusInternalServerError, body: []byte(`{}`)},
			},
			expectedRequests: 1,
		},
		{
			name:   "key set is not JSON",
			issuer: suite.issuerWithURL,
			responses: map[string]fakeResponse{
				testJWKSURL: {status: http.StatusOK, body: []byte(`not json`)},
			},
			expectedRequests: 1,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.transport.requests = nil
			suite.transport.responses = testCase.responses

			_, err := suite.downloader.GetKey(context.Background(), testCase.issuer, "rsa-key")
			suite.Require().Error(err)
			suite.Require().Len(suite.transport.requests, testCase.expectedRequests)
		})
	}
}

// --- test helpers (private, below public functions) ---

func (suite *JWKSDownloaderTestSuite) serveDiscovery(jwksURI string) {
	body, err := json.Marshal(openIDConfiguration{JwksURI: jwksURI})
	suite.Require().NoError(err)
	suite.transport.responses[testDiscoveryURL] = fakeResponse{status: http.StatusOK, body: body}
}

func (suite *JWKSDownloaderTestSuite) serveKeys(rawKeys ...json.RawMessage) {
	body, err := json.Marshal(keySet{Keys: rawKeys})
	suite.Require().NoError(err)
	suite.transport.responses[testJWKSURL] = fakeResponse{status: http.StatusOK, body: body}
}

// webKey builds the JSON of a web key for key (as go-jose writes it), with extras added on top. Passing a
// private key or a []byte builds a private or symmetric web key.
func (suite *JWKSDownloaderTestSuite) webKey(kid string, key interface{}, extras extraFields) json.RawMessage {
	marshaled, err := json.Marshal(gojose.JSONWebKey{Key: key, KeyID: kid})
	suite.Require().NoError(err)

	fields := extraFields{}
	suite.Require().NoError(json.Unmarshal(marshaled, &fields))
	for name, value := range extras {
		fields[name] = value
	}

	merged, err := json.Marshal(fields)
	suite.Require().NoError(err)
	return merged
}

func TestJWKSDownloaderTestSuite(t *testing.T) {
	suite.Run(t, new(JWKSDownloaderTestSuite))
}
