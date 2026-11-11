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

// Package jwtvalidator validates JWTs issued by an external identity provider for the auth-proxy sidecar.
package jwtvalidator

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IssuerConfig is the content of issuer.json. The issuer name is not part of the file.
type IssuerConfig struct {
	IssuerURL     string          `json:"issuer_url"`
	JWKSURL       string          `json:"jwks_url,omitempty"`
	Audiences     []string        `json:"audiences,omitempty"`
	IdentityClaim string          `json:"identity_claim,omitempty"` // default "sub"
	JWKSCacheTTL  metav1.Duration `json:"jwks_cache_ttl,omitempty"`
}

// IssuerConfigReloader holds the issuer entry loaded from the mounted issuer file and picks up changes on request,
// with no goroutine: a request checks the file at most once per check interval.
type IssuerConfigReloader struct {
	logger        logger.Logger
	path          string
	checkInterval time.Duration

	current     atomic.Pointer[IssuerConfig]
	lastChecked atomic.Int64 // unix nanoseconds

	// guards the fields below; held only by the request that won the check
	mutex       sync.Mutex
	checkedInfo os.FileInfo
}

// NewIssuerConfigReloader creates the reloader and tries one load. A missing or invalid file is not an error
// here: it is logged, and the file is picked up by a later Refresh once it is valid.
func NewIssuerConfigReloader(ctx context.Context,
	parentLogger logger.Logger,
	path string,
	checkInterval time.Duration) *IssuerConfigReloader {

	reloader := &IssuerConfigReloader{
		logger:        parentLogger.GetChild("issuer-config-reloader"),
		path:          path,
		checkInterval: checkInterval,
	}

	reloader.lastChecked.Store(time.Now().UnixNano())
	reloader.reload(ctx)
	return reloader
}

// Validate checks that the URLs are absolute https URLs.
func (c *IssuerConfig) Validate() error {
	if err := validateHTTPSURL(c.IssuerURL); err != nil {
		return errors.Wrap(err, "Invalid issuer_url")
	}

	if c.JWKSURL != "" {
		if err := validateHTTPSURL(c.JWKSURL); err != nil {
			return errors.Wrap(err, "Invalid jwks_url")
		}
	}

	return nil
}

// Get returns the current issuer entry, or nil when no valid file was loaded yet. The entry is never
// modified after it is published, so a request keeps using the one it got.
func (s *IssuerConfigReloader) Get() *IssuerConfig {
	return s.current.Load()
}

// Refresh reloads the file when it changed. It returns at once unless the check interval has passed, and
// only one concurrent caller does the check.
func (s *IssuerConfigReloader) Refresh(ctx context.Context) {
	now := time.Now().UnixNano()
	last := s.lastChecked.Load()
	if now-last < s.checkInterval.Nanoseconds() {
		return
	}
	if !s.lastChecked.CompareAndSwap(last, now) {
		return
	}

	s.reload(ctx)
}

func (s *IssuerConfigReloader) reload(ctx context.Context) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// the kubelet swaps the ..data symlink to publish a new version, which makes the path resolve to a
	// different file, so comparing the file identity and its modification time sees every update
	info, err := os.Stat(s.path)
	if err != nil {
		s.logger.WarnWithCtx(ctx, "Failed to stat issuer entry", "path", s.path, "err", err.Error())
		return
	}

	if s.checkedInfo != nil && os.SameFile(s.checkedInfo, info) && s.checkedInfo.ModTime().Equal(info.ModTime()) {
		return
	}

	content, err := os.ReadFile(s.path)
	if err != nil {
		s.logger.WarnWithCtx(ctx, "Failed to read issuer entry", "path", s.path, "err", err.Error())
		return
	}
	s.checkedInfo = info

	issuerConfig, err := parseIssuerConfig(content)
	if err != nil {
		s.logger.WarnWithCtx(ctx, "Issuer entry rejected, keeping last good", "path", s.path, "err", err.Error())
		return
	}

	previous := s.current.Load()
	changed := changedFields(previous, issuerConfig)

	// a new file can hold the same entry, e.g. a ConfigMap re-applied with the same values but different
	// whitespace or key order: `{"issuer_url":"https://idp"}` vs `{ "issuer_url": "https://idp" }`
	if previous != nil && len(changed) == 0 {
		return
	}

	if previous == nil {
		s.logger.InfoWithCtx(ctx, "Issuer entry loaded", "fields", changed)
	} else {
		s.logger.InfoWithCtx(ctx, "Issuer entry reloaded", "changedFields", changed)
	}

	s.current.Store(issuerConfig)
}

func parseIssuerConfig(content []byte) (*IssuerConfig, error) {
	issuerConfig := &IssuerConfig{}
	if err := json.Unmarshal(content, issuerConfig); err != nil {
		return nil, errors.Wrap(err, "Failed to parse issuer entry")
	}

	if err := issuerConfig.Validate(); err != nil {
		return nil, errors.Wrap(err, "Failed to validate issuer entry")
	}

	return issuerConfig, nil
}

// changedFields maps the JSON name of each field that differs to its new value. A nil previous (the first
// load) is compared as an empty entry, so every field that is set is listed.
func changedFields(previous, current *IssuerConfig) map[string]interface{} {
	if previous == nil {
		previous = &IssuerConfig{}
	}

	changed := map[string]interface{}{}
	if previous.IssuerURL != current.IssuerURL {
		changed["issuer_url"] = current.IssuerURL
	}
	if previous.JWKSURL != current.JWKSURL {
		changed["jwks_url"] = current.JWKSURL
	}
	if !slices.Equal(previous.Audiences, current.Audiences) {
		changed["audiences"] = current.Audiences
	}
	if previous.IdentityClaim != current.IdentityClaim {
		changed["identity_claim"] = current.IdentityClaim
	}
	if previous.JWKSCacheTTL != current.JWKSCacheTTL {
		changed["jwks_cache_ttl"] = current.JWKSCacheTTL.Duration.String()
	}
	return changed
}

func validateHTTPSURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse URL '%s'", rawURL)
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return errors.Errorf("URL must be an absolute https URL: %s", rawURL)
	}
	return nil
}
