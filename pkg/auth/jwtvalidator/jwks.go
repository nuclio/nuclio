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
	"crypto/rsa"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/auth/configReloader"

	gojose "github.com/go-jose/go-jose/v4"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

const (
	downloadTimeout    = 5 * time.Second
	maxDownloadBody    = 1024 * 1024
	maxRedirects       = 3
	minRSAKeyBits      = 2048
	discoveryPath      = "/.well-known/openid-configuration"
	signingKeyUse      = "sig"
	verifyKeyOperation = "verify"
)

// JWKSDownloader is a KeyProvider that downloads the identity provider's key set on every call:
// it keeps no keys and shares nothing between calls.
type JWKSDownloader struct {
	logger     logger.Logger
	httpClient *http.Client
}

// NewJWKSDownloader creates a downloader that reaches the identity provider over https only.
func NewJWKSDownloader(parentLogger logger.Logger) *JWKSDownloader {
	return &JWKSDownloader{
		logger: parentLogger.GetChild("jwks-downloader"),
		httpClient: &http.Client{
			CheckRedirect: func(request *http.Request, via []*http.Request) error {
				if len(via) > maxRedirects {
					return errors.New("Too many redirects")
				}
				return validateHTTPS(request.URL)
			},
		},
	}
}

// GetKey downloads the key set of the issuer and returns the usable key with the given key ID.
func (d *JWKSDownloader) GetKey(ctx context.Context, issuer *configReloader.Config, kid string) (*Key, error) {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	jwksURL := issuer.JwksURL
	if jwksURL == "" {
		var err error
		jwksURL, err = d.discoverJWKSURL(ctx, issuer.IssuerURL)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to discover key set URL")
		}
	}

	d.logger.DebugWithCtx(ctx, "Downloading key set", "issuerURL", issuer.IssuerURL, "jwksURL", jwksURL, "kid", kid)

	var downloaded keySet
	if err := d.getJSON(ctx, jwksURL, &downloaded); err != nil {
		return nil, errors.Wrap(err, "Failed to download key set")
	}

	keys := parseUsableKeys(downloaded.Keys)
	d.logger.DebugWithCtx(ctx,
		"Downloaded key set",
		"jwksURL", jwksURL,
		"keyCount", len(downloaded.Keys),
		"usableKeyIDs", slices.Collect(maps.Keys(keys)))

	if len(keys) == 0 {
		return nil, errors.Wrap(ErrNoKey, "Key set has no usable key")
	}

	key, found := keys[kid]
	if !found {
		return nil, errors.Wrapf(ErrNoKey, "Key set has no usable key for the key ID (kid=%s)", kid)
	}
	return key, nil
}

func (d *JWKSDownloader) discoverJWKSURL(ctx context.Context, issuerURL string) (string, error) {
	var configuration openIDConfiguration
	if err := d.getJSON(ctx, strings.TrimSuffix(issuerURL, "/")+discoveryPath, &configuration); err != nil {
		return "", errors.Wrap(err, "Failed to download discovery document")
	}

	parsedURL, err := url.Parse(configuration.JwksURI)
	if err != nil {
		return "", errors.Wrap(err, "Failed to parse key set URL from discovery document")
	}
	if err := validateHTTPS(parsedURL); err != nil {
		return "", errors.Wrap(err, "Invalid key set URL in discovery document")
	}
	return configuration.JwksURI, nil
}

func (d *JWKSDownloader) getJSON(ctx context.Context, rawURL string, target interface{}) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return errors.Wrap(err, "Failed to parse URL")
	}
	if err := validateHTTPS(parsedURL); err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to create request")
	}

	response, err := d.httpClient.Do(request)
	if err != nil {
		return errors.Wrap(err, "Failed to send request")
	}
	defer response.Body.Close() // nolint: errcheck

	if response.StatusCode != http.StatusOK {
		return errors.Errorf("Unexpected response status: %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxDownloadBody+1))
	if err != nil {
		return errors.Wrap(err, "Failed to read response body")
	}
	if len(body) > maxDownloadBody {
		return errors.New("Response body is too large")
	}

	if err := json.Unmarshal(body, target); err != nil {
		return errors.Wrap(err, "Failed to parse response body")
	}
	return nil
}

// validateHTTPS rejects any URL that is not https. It is checked for every URL the downloader is about to
// fetch and for each redirect.
func validateHTTPS(parsedURL *url.URL) error {
	if parsedURL.Scheme != "https" {
		return errors.Errorf("URL must use https (scheme=%s)", parsedURL.Scheme)
	}
	return nil
}

// parseUsableKeys returns the signing keys that are safe to verify with, by key ID. Each key is parsed on its
// own, so a key that is malformed or unusable is skipped without failing the rest of the set.
func parseUsableKeys(rawKeys []json.RawMessage) map[string]*Key {
	keys := map[string]*Key{}
	for _, rawKey := range rawKeys {
		var webKey gojose.JSONWebKey
		if err := json.Unmarshal(rawKey, &webKey); err != nil {
			continue
		}

		var operations keyOperations
		if err := json.Unmarshal(rawKey, &operations); err != nil {
			continue
		}

		if key, usable := toUsableKey(&webKey, operations.KeyOps); usable {
			keys[webKey.KeyID] = key
		}
	}
	return keys
}

// toUsableKey returns the key when it is a public signing key that fits the allowed algorithms.
func toUsableKey(webKey *gojose.JSONWebKey, keyOps []string) (*Key, bool) {
	if !kidPattern.MatchString(webKey.KeyID) {
		return nil, false
	}
	if webKey.Use != "" && webKey.Use != signingKeyUse {
		return nil, false
	}
	if len(keyOps) > 0 && !slices.Contains(keyOps, verifyKeyOperation) {
		return nil, false
	}
	if webKey.Algorithm != "" && !isAllowedAlgorithm(webKey.Algorithm) {
		return nil, false
	}

	// anything but a public RSA or EC key (a private key, a symmetric key) is not accepted
	switch publicKey := webKey.Key.(type) {
	case *rsa.PublicKey:
		fitsAlgorithm := webKey.Algorithm == "" ||
			webKey.Algorithm == string(gojose.RS256) ||
			webKey.Algorithm == string(gojose.PS256)
		if !fitsAlgorithm || publicKey.N.BitLen() < minRSAKeyBits {
			return nil, false
		}
	case *ecdsa.PublicKey:
		fitsAlgorithm := webKey.Algorithm == "" || webKey.Algorithm == string(gojose.ES256)
		if !fitsAlgorithm || publicKey.Curve != elliptic.P256() {
			return nil, false
		}
	default:
		return nil, false
	}
	return &Key{PublicKey: webKey.Key, Algorithm: webKey.Algorithm}, true
}
