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
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/auth/configReloader"
	"github.com/nuclio/nuclio/pkg/common/testutils"

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

var testCacheConfig = &CacheConfig{KeyTTL: 10 * time.Minute, RejectedKidTTL: 30 * time.Second, RejectedKidCacheSize: 1000}

type JWKSDownloaderTestSuite struct {
	suite.Suite
	logger        logger.Logger
	rsaKey        *rsa.PrivateKey
	weakRSAKey    *rsa.PrivateKey
	ecKey         *ecdsa.PrivateKey
	downloader    *JWKSDownloader
	issuer        *configReloader.Config
	issuerWithURL *configReloader.Config
}

// fakeIdentityProvider counts the requests and holds the key set response until released. It is a separate
// value per test, so a download still running when its test ends touches nothing of the next test.
type fakeIdentityProvider struct {
	discoveryRequests atomic.Int32
	keySetRequests    atomic.Int32
	keySetRequested   chan struct{}
	releaseKeySet     chan struct{}
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
	suite.issuer = &configReloader.Config{IssuerURL: testIssuerURL}
	suite.issuerWithURL = &configReloader.Config{IssuerURL: testIssuerURL, JwksURL: testJWKSURL}
}

func (suite *JWKSDownloaderTestSuite) SetupTest() {
	suite.downloader = NewJWKSDownloader(suite.logger, testCacheConfig)
}

func (suite *JWKSDownloaderTestSuite) TestConcurrentCallsShareOneDownload() {
	callerCount := 200
	provider := suite.useFakeIdentityProvider()

	errorsChan := make(chan error, callerCount)
	var wg sync.WaitGroup
	for range callerCount {
		wg.Go(func() {
			_, err := suite.downloader.GetKey(context.Background(), suite.issuer, "rsa-key")
			errorsChan <- err
		})
	}

	// the download is blocked until every caller had the chance to join it; a caller that joins late would
	// start a second download, which the request counts below would show
	<-provider.keySetRequested
	time.Sleep(200 * time.Millisecond)
	close(provider.releaseKeySet)
	wg.Wait()
	close(errorsChan)

	for err := range errorsChan {
		suite.Require().NoError(err)
	}
	suite.Require().Equal(int32(1), provider.discoveryRequests.Load())
	suite.Require().Equal(int32(1), provider.keySetRequests.Load())
}

func (suite *JWKSDownloaderTestSuite) TestCancelledCallerDoesNotFailOthers() {
	provider := suite.useFakeIdentityProvider()
	firstCtx, cancelFirst := context.WithCancel(context.Background())
	defer cancelFirst()

	firstErrorChan := make(chan error, 1)
	go func() {
		_, err := suite.downloader.GetKey(firstCtx, suite.issuerWithURL, "rsa-key")
		firstErrorChan <- err
	}()
	<-provider.keySetRequested

	type result struct {
		key *Key
		err error
	}
	secondResultChan := make(chan result, 1)
	go func() {
		key, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "rsa-key")
		secondResultChan <- result{key: key, err: err}
	}()

	// the second caller must join the download before the first one gives up, otherwise it would start its own
	time.Sleep(200 * time.Millisecond)

	// the first caller gives up while the download is still blocked, and returns without waiting for it
	cancelFirst()
	suite.Require().ErrorIs(<-firstErrorChan, context.Canceled)

	close(provider.releaseKeySet)
	secondResult := <-secondResultChan
	suite.Require().NoError(secondResult.err)
	suite.Require().Equal(&suite.rsaKey.PublicKey, secondResult.key.PublicKey)
	suite.Require().Equal(int32(1), provider.keySetRequests.Load())
}

func (suite *JWKSDownloaderTestSuite) TestDifferentKeySourcesDoNotShareDownload() {
	provider := suite.useFakeIdentityProvider()

	errorsChan := make(chan error, 2)
	for _, issuer := range []*configReloader.Config{suite.issuer, suite.issuerWithURL} {
		go func() {
			_, err := suite.downloader.GetKey(context.Background(), issuer, "rsa-key")
			errorsChan <- err
		}()
	}

	// each key source starts its own download, so both requests arrive while the key set is still held
	<-provider.keySetRequested
	<-provider.keySetRequested
	close(provider.releaseKeySet)

	suite.Require().NoError(<-errorsChan)
	suite.Require().NoError(<-errorsChan)
	suite.Require().Equal(int32(2), provider.keySetRequests.Load())
}

func (suite *JWKSDownloaderTestSuite) TestCallerStopsAtItsOwnDeadline() {
	provider := suite.useFakeIdentityProvider()
	// the deadline has already passed, so the outcome does not depend on timing: the download is blocked, and
	// the caller must still return at its own deadline
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, err := suite.downloader.GetKey(ctx, suite.issuerWithURL, "rsa-key")
	suite.Require().ErrorIs(err, context.DeadlineExceeded)
	close(provider.releaseKeySet)
}

func (suite *JWKSDownloaderTestSuite) TestUnknownKeyIdIsNoKey() {
	provider := suite.useFakeIdentityProvider()
	close(provider.releaseKeySet)

	_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "other-key")
	suite.Require().ErrorIs(err, ErrNoKey)
}

func (suite *JWKSDownloaderTestSuite) TestDiscoversKeySetAndReturnsKeys() {
	discoveryBody := suite.discoveryBody(testJWKSURL)
	keySetBody := suite.keySetBody(
		suite.webKey("rsa-key", &suite.rsaKey.PublicKey, map[string]interface{}{"alg": "RS256"}),
		suite.webKey("ec-key", &suite.ecKey.PublicKey, map[string]interface{}{"use": "sig"}),
	)
	suite.downloader.httpClient = testutils.CreateDummyHTTPClient(func(r *http.Request) *http.Response {
		switch r.URL.String() {
		case testDiscoveryURL:
			return suite.newResponse(http.StatusOK, discoveryBody)
		case testJWKSURL:
			return suite.newResponse(http.StatusOK, keySetBody)
		default:
			suite.Failf("Unexpected request", "URL: %s", r.URL)
			return nil
		}
	})

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
	discoveryBody := suite.discoveryBody(testJWKSURL)
	keySetBody := suite.keySetBody(suite.webKey("rsa-key", &suite.rsaKey.PublicKey, nil))
	var requestedURLs []string
	suite.downloader.httpClient = testutils.CreateDummyHTTPClient(func(r *http.Request) *http.Response {
		requestedURLs = append(requestedURLs, r.URL.String())
		switch r.URL.String() {
		case testDiscoveryURL:
			return suite.newResponse(http.StatusOK, discoveryBody)
		case testJWKSURL:
			return suite.newResponse(http.StatusOK, keySetBody)
		default:
			suite.Failf("Unexpected request", "URL: %s", r.URL)
			return nil
		}
	})

	issuer := &configReloader.Config{IssuerURL: testIssuerURL + "/"}
	_, err := suite.downloader.GetKey(context.Background(), issuer, "rsa-key")
	suite.Require().NoError(err)
	suite.Require().Equal([]string{testDiscoveryURL, testJWKSURL}, requestedURLs)
}

func (suite *JWKSDownloaderTestSuite) TestConfiguredKeySetUrlSkipsDiscovery() {
	keySetBody := suite.keySetBody(suite.webKey("rsa-key", &suite.rsaKey.PublicKey, nil))
	var requestedURLs []string
	suite.downloader.httpClient = testutils.CreateDummyHTTPClient(func(r *http.Request) *http.Response {
		requestedURLs = append(requestedURLs, r.URL.String())
		switch r.URL.String() {
		case testJWKSURL:
			return suite.newResponse(http.StatusOK, keySetBody)
		default:
			suite.Failf("Unexpected request", "URL: %s", r.URL)
			return nil
		}
	})

	_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "rsa-key")
	suite.Require().NoError(err)
	suite.Require().Equal([]string{testJWKSURL}, requestedURLs)
}

func (suite *JWKSDownloaderTestSuite) TestUnusableKeysAreSkipped() {
	keySetBody := suite.keySetBody(
		suite.webKey("good", &suite.rsaKey.PublicKey, nil),
		suite.webKey("encryption", &suite.rsaKey.PublicKey, map[string]interface{}{"use": "enc"}),
		suite.webKey("no-verify", &suite.rsaKey.PublicKey, map[string]interface{}{"key_ops": []string{"encrypt"}}),
		suite.webKey("empty-ops", &suite.rsaKey.PublicKey, map[string]interface{}{"key_ops": []string{}}),
		suite.webKey("weak", &suite.weakRSAKey.PublicKey, nil),
		suite.webKey("bad-alg", &suite.rsaKey.PublicKey, map[string]interface{}{"alg": "HS256"}),
		suite.webKey("wrong-type-alg", &suite.rsaKey.PublicKey, map[string]interface{}{"alg": "ES256"}),
		suite.webKey("ec-wrong-alg", &suite.ecKey.PublicKey, map[string]interface{}{"alg": "RS256"}),
		suite.webKey("bad kid", &suite.rsaKey.PublicKey, nil),
		suite.webKey("private", suite.rsaKey, nil),
		suite.webKey("symmetric", []byte("shared-secret"), nil),
		json.RawMessage(`{"kty":"RSA","kid":"broken","n":"!!","e":"AQAB"}`),
	)
	suite.downloader.httpClient = testutils.CreateDummyHTTPClient(func(r *http.Request) *http.Response {
		switch r.URL.String() {
		case testJWKSURL:
			return suite.newResponse(http.StatusOK, keySetBody)
		default:
			suite.Failf("Unexpected request", "URL: %s", r.URL)
			return nil
		}
	})

	for _, testCase := range []struct {
		name     string
		kid      string
		expectOK bool
	}{
		{name: "usable key", kid: "good", expectOK: true},
		{name: "use is enc", kid: "encryption"},
		{name: "key_ops lacks verify", kid: "no-verify"},
		{name: "key_ops is empty", kid: "empty-ops"},
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
	keySetBody := suite.keySetBody(suite.webKey("weak", &suite.weakRSAKey.PublicKey, nil))
	suite.downloader.httpClient = testutils.CreateDummyHTTPClient(func(r *http.Request) *http.Response {
		switch r.URL.String() {
		case testJWKSURL:
			return suite.newResponse(http.StatusOK, keySetBody)
		default:
			suite.Failf("Unexpected request", "URL: %s", r.URL)
			return nil
		}
	})

	_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "weak")
	suite.Require().True(errors.Is(err, ErrNoKey), errors.GetErrorStackString(err, -1))
}

func (suite *JWKSDownloaderTestSuite) TestParseUsableKeys() {
	otherCurveKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	suite.Require().NoError(err)
	maxRSAKey, err := rsa.GenerateKey(rand.Reader, gojose.MaxRSAModulusBits)
	suite.Require().NoError(err)

	// a modulus one bit above the library limit, no real key needed since only its size is checked
	oversizedModulus := base64.RawURLEncoding.EncodeToString(new(big.Int).Lsh(big.NewInt(1), uint(gojose.MaxRSAModulusBits)).Bytes())

	for _, testCase := range []struct {
		name               string
		rawKeys            []json.RawMessage
		expectedAlgorithms map[string]string // usable key ID -> algorithm the key declares
		expectError        bool
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
				suite.webKey("rsa-rs256", &suite.rsaKey.PublicKey, map[string]interface{}{"alg": "RS256"}),
				suite.webKey("rsa-ps256", &suite.rsaKey.PublicKey, map[string]interface{}{"alg": "PS256"}),
				suite.webKey("ec", &suite.ecKey.PublicKey, nil),
				suite.webKey("ec-es256", &suite.ecKey.PublicKey, map[string]interface{}{"alg": "ES256"}),
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
				suite.webKey("sign-and-verify", &suite.rsaKey.PublicKey, map[string]interface{}{"key_ops": []string{"sign", "verify"}}),
			},
			expectedAlgorithms: map[string]string{"sign-and-verify": ""},
		},
		{
			name: "key_ops that is not a list",
			rawKeys: []json.RawMessage{
				suite.webKey("ops-string", &suite.rsaKey.PublicKey, map[string]interface{}{"key_ops": "verify"}),
			},
			expectedAlgorithms: map[string]string{},
		},
		{
			name: "RSA key at the largest size the library parses",
			rawKeys: []json.RawMessage{
				suite.webKey("max-rsa", &maxRSAKey.PublicKey, nil),
			},
			expectedAlgorithms: map[string]string{"max-rsa": ""},
		},
		{
			name: "RSA key above the largest size the library parses",
			rawKeys: []json.RawMessage{
				json.RawMessage(`{"kty":"RSA","kid":"too-big","n":"` + oversizedModulus + `","e":"AQAB"}`),
			},
			expectedAlgorithms: map[string]string{},
		},
		{
			name: "the same key published twice",
			rawKeys: []json.RawMessage{
				suite.webKey("twice", &suite.rsaKey.PublicKey, nil),
				suite.webKey("twice", &suite.rsaKey.PublicKey, nil),
			},
			expectedAlgorithms: map[string]string{"twice": ""},
		},
		{
			name: "different keys under one key ID",
			rawKeys: []json.RawMessage{
				suite.webKey("clash", &suite.rsaKey.PublicKey, nil),
				suite.webKey("clash", &suite.ecKey.PublicKey, nil),
				suite.webKey("fine", &suite.rsaKey.PublicKey, nil),
			},
			expectError: true,
		},
		{
			name: "a usable and an unusable key under one key ID",
			rawKeys: []json.RawMessage{
				suite.webKey("both", &suite.rsaKey.PublicKey, map[string]interface{}{"use": "enc"}),
				suite.webKey("both", &suite.rsaKey.PublicKey, nil),
			},
			expectedAlgorithms: map[string]string{"both": ""},
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
		{
			name: "keys that are not signing keys, and a key ID that is not valid",
			rawKeys: []json.RawMessage{
				suite.webKey("encryption", &suite.rsaKey.PublicKey, map[string]interface{}{"use": "enc"}),
				suite.webKey("bad-alg", &suite.rsaKey.PublicKey, map[string]interface{}{"alg": "HS256"}),
				suite.webKey("bad kid", &suite.rsaKey.PublicKey, nil),
			},
			expectedAlgorithms: map[string]string{},
		},
	} {
		suite.Run(testCase.name, func() {
			usableKeys, err := parseUsableKeys(testCase.rawKeys)
			if testCase.expectError {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)

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
		kid              string
		responseURL      string // the only URL the identity provider answers
		response         *http.Response
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
			name:             "discovery document points to a URL that is not https",
			issuer:           suite.issuer,
			responseURL:      testDiscoveryURL,
			response:         suite.newResponse(http.StatusOK, []byte(`{"issuer":"`+testIssuerURL+`","jwks_uri":"http://idp.example.com/certs"}`)),
			expectedRequests: 1,
		},
		{
			name:             "discovery document of another issuer",
			issuer:           suite.issuer,
			responseURL:      testDiscoveryURL,
			response:         suite.newResponse(http.StatusOK, []byte(`{"issuer":"https://idp.example.com/realms/other","jwks_uri":"`+testJWKSURL+`"}`)),
			expectedRequests: 1,
		},
		{
			name:             "discovery document without an issuer",
			issuer:           suite.issuer,
			responseURL:      testDiscoveryURL,
			response:         suite.newResponse(http.StatusOK, []byte(`{"jwks_uri":"`+testJWKSURL+`"}`)),
			expectedRequests: 1,
		},
		{
			name:             "key ID with invalid characters",
			issuer:           suite.issuerWithURL,
			kid:              "bad kid",
			expectedRequests: 0,
		},
		{
			name:             "key set body over 1 MiB",
			issuer:           suite.issuerWithURL,
			responseURL:      testJWKSURL,
			response:         suite.newResponse(http.StatusOK, oversizedBody),
			expectedRequests: 1,
		},
		{
			name:             "key set answers with an error status",
			issuer:           suite.issuerWithURL,
			responseURL:      testJWKSURL,
			response:         suite.newResponse(http.StatusInternalServerError, []byte(`{}`)),
			expectedRequests: 1,
		},
		{
			name:             "key set is not JSON",
			issuer:           suite.issuerWithURL,
			responseURL:      testJWKSURL,
			response:         suite.newResponse(http.StatusOK, []byte(`not json`)),
			expectedRequests: 1,
		},
	} {
		suite.Run(testCase.name, func() {
			requestCount := 0
			suite.downloader.httpClient = testutils.CreateDummyHTTPClient(func(r *http.Request) *http.Response {
				requestCount++
				switch r.URL.String() {
				case testCase.responseURL:
					return testCase.response
				default:
					suite.Failf("Unexpected request", "URL: %s", r.URL)
					return nil
				}
			})

			kid := testCase.kid
			if kid == "" {
				kid = "rsa-key"
			}
			_, err := suite.downloader.GetKey(context.Background(), testCase.issuer, kid)
			suite.Require().Error(err)
			suite.Require().Equal(testCase.expectedRequests, requestCount)
		})
	}
}

func (suite *JWKSDownloaderTestSuite) TestCachedKeyIsServedWithoutDownload() {
	provider := suite.useFakeIdentityProvider()
	close(provider.releaseKeySet)

	for range 3 {
		key, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "rsa-key")
		suite.Require().NoError(err)
		suite.Require().Equal(&suite.rsaKey.PublicKey, key.PublicKey)
	}
	suite.Require().Equal(int32(1), provider.keySetRequests.Load())
}

func (suite *JWKSDownloaderTestSuite) TestKeyIsDownloadedAgainAfterItsTTL() {
	for _, testCase := range []struct {
		name             string
		issuerTTL        time.Duration
		keyAge           time.Duration
		expectedRequests int32
	}{
		{name: "younger than the platform TTL", keyAge: 5 * time.Minute, expectedRequests: 1},
		{name: "older than the platform TTL", keyAge: 15 * time.Minute, expectedRequests: 2},
		{name: "older than the issuer's own TTL", issuerTTL: time.Minute, keyAge: 5 * time.Minute, expectedRequests: 2},
		{name: "younger than the issuer's own TTL", issuerTTL: time.Hour, keyAge: 15 * time.Minute, expectedRequests: 1},
	} {
		suite.Run(testCase.name, func() {
			suite.SetupTest()
			provider := suite.useFakeIdentityProvider()
			close(provider.releaseKeySet)
			issuer := &configReloader.Config{JwksURL: testJWKSURL}
			issuer.JwksCacheTTL.Duration = testCase.issuerTTL

			_, err := suite.downloader.GetKey(context.Background(), issuer, "rsa-key")
			suite.Require().NoError(err)
			suite.ageCachedKeys(testCase.keyAge)

			key, err := suite.downloader.GetKey(context.Background(), issuer, "rsa-key")
			suite.Require().NoError(err)
			suite.Require().Equal(&suite.rsaKey.PublicKey, key.PublicKey)
			suite.Require().Equal(testCase.expectedRequests, provider.keySetRequests.Load())
		})
	}
}

func (suite *JWKSDownloaderTestSuite) TestExpiredKeyIsNotUsedWhenDownloadFails() {
	provider := suite.useFakeIdentityProvider()
	close(provider.releaseKeySet)
	_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "rsa-key")
	suite.Require().NoError(err)
	suite.ageCachedKeys(time.Hour)

	suite.downloader.httpClient = testutils.CreateDummyHTTPClient(func(r *http.Request) *http.Response {
		return suite.newResponse(http.StatusInternalServerError, []byte(`{}`))
	})

	_, err = suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "rsa-key")
	suite.Require().Error(err)
	suite.Require().False(errors.Is(err, ErrNoKey), errors.GetErrorStackString(err, -1))
}

func (suite *JWKSDownloaderTestSuite) TestKeyIdWithNoKeyIsRejectedUntilItExpires() {
	provider := suite.useFakeIdentityProvider()
	close(provider.releaseKeySet)

	for range 100 {
		_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "unknown")
		suite.Require().ErrorIs(err, ErrNoKey)
	}
	suite.Require().Equal(int32(1), provider.keySetRequests.Load())

	suite.expireRejectedKid("unknown")
	_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "unknown")
	suite.Require().ErrorIs(err, ErrNoKey)
	suite.Require().Equal(int32(2), provider.keySetRequests.Load())
}

func (suite *JWKSDownloaderTestSuite) TestUnusableKeyIdIsRejectedAfterItsFirstDownload() {
	provider := suite.useFakeIdentityProvider(
		suite.webKey("good", &suite.rsaKey.PublicKey, nil),
		suite.webKey("encryption", &suite.rsaKey.PublicKey, map[string]interface{}{"use": "enc"}),
		suite.webKey("weak", &suite.weakRSAKey.PublicKey, nil),
	)
	close(provider.releaseKeySet)

	_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "good")
	suite.Require().NoError(err)
	suite.Require().Equal(int32(1), provider.keySetRequests.Load())

	for _, kid := range []string{"encryption", "weak"} {
		for range 3 {
			_, err = suite.downloader.GetKey(context.Background(), suite.issuerWithURL, kid)
			suite.Require().ErrorIs(err, ErrNoKey)
		}
	}

	// one download for the good key, and one for each unusable key ID the first time it is asked for
	suite.Require().Equal(int32(3), provider.keySetRequests.Load())
}

func (suite *JWKSDownloaderTestSuite) TestUnknownKeyIdsDoNotEvictGoodKeys() {
	provider := suite.useFakeIdentityProvider()
	close(provider.releaseKeySet)

	_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "rsa-key")
	suite.Require().NoError(err)

	for i := range 50 {
		_, err = suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "random-"+strconv.Itoa(i))
		suite.Require().ErrorIs(err, ErrNoKey)
	}
	requestsBefore := provider.keySetRequests.Load()

	key, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "rsa-key")
	suite.Require().NoError(err)
	suite.Require().Equal(&suite.rsaKey.PublicKey, key.PublicKey)
	suite.Require().Equal(requestsBefore, provider.keySetRequests.Load())
}

func (suite *JWKSDownloaderTestSuite) TestKnownKeyIsNotBlockedByRunningDownload() {
	provider := suite.useFakeIdentityProvider()
	suite.downloader.keys.Store("rsa-key", &keyEntry{
		key:          &Key{PublicKey: &suite.rsaKey.PublicKey},
		downloadedAt: time.Now(),
	})

	blockedErrorChan := make(chan error, 1)
	go func() {
		_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "unknown")
		blockedErrorChan <- err
	}()
	<-provider.keySetRequested

	key, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "rsa-key")
	suite.Require().NoError(err)
	suite.Require().Equal(&suite.rsaKey.PublicKey, key.PublicKey)

	close(provider.releaseKeySet)
	suite.Require().ErrorIs(<-blockedErrorChan, ErrNoKey)
}

func (suite *JWKSDownloaderTestSuite) TestFailedDownloadKeepsCachedKeysAndDoesNotMarkKeyIdAsBad() {
	provider := suite.useFakeIdentityProvider(suite.webKey("weak", &suite.weakRSAKey.PublicKey, nil))
	close(provider.releaseKeySet)
	suite.downloader.keys.Store("cached", &keyEntry{
		key:          &Key{PublicKey: &suite.rsaKey.PublicKey},
		downloadedAt: time.Now(),
	})

	for range 2 {
		_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "unknown")
		suite.Require().ErrorIs(err, ErrNoKey)
	}
	suite.Require().Equal(int32(2), provider.keySetRequests.Load())

	key, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "cached")
	suite.Require().NoError(err)
	suite.Require().Equal(&suite.rsaKey.PublicKey, key.PublicKey)
	suite.Require().Equal(int32(2), provider.keySetRequests.Load())
}

func (suite *JWKSDownloaderTestSuite) TestKeysTheIdentityProviderDroppedAreRemoved() {
	provider := suite.useFakeIdentityProvider()
	close(provider.releaseKeySet)
	suite.downloader.keys.Store("dropped", &keyEntry{
		key:          &Key{PublicKey: &suite.rsaKey.PublicKey},
		downloadedAt: time.Now(),
	})

	_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "rsa-key")
	suite.Require().NoError(err)
	suite.Require().Equal(int32(1), provider.keySetRequests.Load())

	_, found := suite.downloader.keys.Load("dropped")
	suite.Require().False(found)
}

func (suite *JWKSDownloaderTestSuite) TestFlushKeysRemovesCachedKeysButNotRejectedKeyIds() {
	provider := suite.useFakeIdentityProvider()
	close(provider.releaseKeySet)

	_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "unknown")
	suite.Require().ErrorIs(err, ErrNoKey)
	_, err = suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "rsa-key")
	suite.Require().NoError(err)
	suite.Require().Equal(int32(1), provider.keySetRequests.Load())

	suite.downloader.FlushKeys()

	key, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "rsa-key")
	suite.Require().NoError(err)
	suite.Require().Equal(&suite.rsaKey.PublicKey, key.PublicKey)
	suite.Require().Equal(int32(2), provider.keySetRequests.Load())

	_, err = suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "unknown")
	suite.Require().ErrorIs(err, ErrNoKey)
	suite.Require().Equal(int32(2), provider.keySetRequests.Load())
}

func (suite *JWKSDownloaderTestSuite) TestFullRejectedKeyIdCacheDoesNotStoreNewKeyIds() {
	suite.downloader.rejectedKids.size = 2
	provider := suite.useFakeIdentityProvider()
	close(provider.releaseKeySet)

	for _, kid := range []string{"first", "second", "third"} {
		_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, kid)
		suite.Require().ErrorIs(err, ErrNoKey)
	}
	suite.Require().Equal(int32(3), provider.keySetRequests.Load())

	// the two stored key IDs are rejected from the cache, the one that did not fit downloads again
	for _, kid := range []string{"first", "second", "third"} {
		_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, kid)
		suite.Require().ErrorIs(err, ErrNoKey)
	}
	suite.Require().Equal(int32(4), provider.keySetRequests.Load())
}

func (suite *JWKSDownloaderTestSuite) TestFullRejectedKeyIdCacheDropsExpiredKeyIdsFirst() {
	suite.downloader.rejectedKids.size = 2
	provider := suite.useFakeIdentityProvider()
	close(provider.releaseKeySet)

	for _, kid := range []string{"first", "second"} {
		_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, kid)
		suite.Require().ErrorIs(err, ErrNoKey)
	}
	suite.expireRejectedKid("first")

	_, err := suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "third")
	suite.Require().ErrorIs(err, ErrNoKey)
	suite.Require().Equal(int32(3), provider.keySetRequests.Load())

	// "third" took the place of the expired "first", so it is now rejected from the cache
	_, err = suite.downloader.GetKey(context.Background(), suite.issuerWithURL, "third")
	suite.Require().ErrorIs(err, ErrNoKey)
	suite.Require().Equal(int32(3), provider.keySetRequests.Load())
}

// --- test helpers (private, below public functions) ---

// useFakeIdentityProvider makes the downloader talk to a fake identity provider that answers discovery at
// once and holds the key set response until its releaseKeySet is closed. It serves rawKeys, or one RSA key
// "rsa-key" when none are given.
func (suite *JWKSDownloaderTestSuite) useFakeIdentityProvider(rawKeys ...json.RawMessage) *fakeIdentityProvider {
	provider := &fakeIdentityProvider{
		keySetRequested: make(chan struct{}, 1000),
		releaseKeySet:   make(chan struct{}),
	}
	discoveryBody := suite.discoveryBody(testJWKSURL)
	if len(rawKeys) == 0 {
		rawKeys = []json.RawMessage{suite.webKey("rsa-key", &suite.rsaKey.PublicKey, nil)}
	}
	keySetBody := suite.keySetBody(rawKeys...)
	suite.downloader.httpClient = testutils.CreateDummyHTTPClient(func(r *http.Request) *http.Response {
		switch r.URL.String() {
		case testDiscoveryURL:
			provider.discoveryRequests.Add(1)
			return suite.newResponse(http.StatusOK, discoveryBody)
		case testJWKSURL:
			provider.keySetRequests.Add(1)
			provider.keySetRequested <- struct{}{}
			<-provider.releaseKeySet
			return suite.newResponse(http.StatusOK, keySetBody)
		default:
			suite.Failf("Unexpected request", "URL: %s", r.URL)
			return nil
		}
	})
	return provider
}

// ageCachedKeys makes every cached key look as if it was downloaded age ago, so a test needs no sleep.
func (suite *JWKSDownloaderTestSuite) ageCachedKeys(age time.Duration) {
	suite.downloader.keys.Range(func(_, value interface{}) bool {
		value.(*keyEntry).downloadedAt = time.Now().Add(-age)
		return true
	})
}

// expireRejectedKid makes the rejected key ID cache entry of kid look expired.
func (suite *JWKSDownloaderTestSuite) expireRejectedKid(kid string) {
	suite.downloader.rejectedKids.Lock()
	defer suite.downloader.rejectedKids.Unlock()
	suite.downloader.rejectedKids.expiresAt[kid] = time.Now().Add(-time.Second)
}

// newResponse builds the response an identity provider gives, with a body that can be read once.
func (suite *JWKSDownloaderTestSuite) newResponse(status int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     http.Header{},
	}
}

func (suite *JWKSDownloaderTestSuite) discoveryBody(jwksURI string) []byte {
	body, err := json.Marshal(openIDConfiguration{Issuer: testIssuerURL, JwksURI: jwksURI})
	suite.Require().NoError(err)
	return body
}

func (suite *JWKSDownloaderTestSuite) keySetBody(rawKeys ...json.RawMessage) []byte {
	body, err := json.Marshal(keySet{Keys: rawKeys})
	suite.Require().NoError(err)
	return body
}

// webKey builds the JSON of a web key for key (as go-jose writes it), with extras added on top. Passing a
// private key or a []byte builds a private or symmetric web key.
func (suite *JWKSDownloaderTestSuite) webKey(kid string, key interface{}, extras map[string]interface{}) json.RawMessage {
	marshaled, err := json.Marshal(gojose.JSONWebKey{Key: key, KeyID: kid})
	suite.Require().NoError(err)

	fields := map[string]interface{}{}
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
