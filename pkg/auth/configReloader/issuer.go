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

// Package configReloader loads the trusted issuer entry (issuer.json) mounted into the auth-proxy sidecar
// and reloads it when it changes.
package configReloader

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

// DefaultIdentityClaim is the token claim used as the caller identity when issuer.json sets none.
const DefaultIdentityClaim = "sub"

// Config is the content of issuer.json.
type Config struct {
	IssuerName    string          `json:"issuer_name"` // used for X-Nuclio-Auth-Issuer
	IssuerURL     string          `json:"issuer_url"`
	JWKSURL       string          `json:"jwks_url,omitempty"`
	Audiences     []string        `json:"audiences,omitempty"`
	IdentityClaim string          `json:"identity_claim,omitempty"`
	JWKSCacheTTL  metav1.Duration `json:"jwks_cache_ttl,omitempty"`
}

// Validate checks that the URLs are absolute https URLs.
func (c *Config) Validate() error {
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

// Reloader holds the issuer entry loaded from the mounted issuer file and picks up changes on request,
// with no goroutine: a request checks the file at most once per check interval.
type Reloader struct {
	logger        logger.Logger
	path          string
	checkInterval time.Duration

	current     atomic.Pointer[Config]
	lastChecked atomic.Int64 // unix nanoseconds

	// guards the fields below; held only by the request that won the check
	mutex       sync.Mutex
	checkedInfo os.FileInfo
}

// NewReloader creates the reloader and loads the file once
func NewReloader(ctx context.Context,
	parentLogger logger.Logger,
	path string,
	checkInterval time.Duration) (*Reloader, error) {

	reloader := &Reloader{
		logger:        parentLogger.GetChild("issuer-config-reloader"),
		path:          path,
		checkInterval: checkInterval,
	}

	reloader.lastChecked.Store(time.Now().UnixNano())
	if err := reloader.reload(ctx); err != nil {
		return nil, errors.Wrap(err, "Failed to load issuer entry")
	}
	return reloader, nil
}

// Get returns the current issuer entry, or nil when no valid file was loaded yet. The entry is never
// modified after it is published, so a request keeps using the one it got.
func (r *Reloader) Get() *Config {
	return r.current.Load()
}

// Refresh reloads the file when it changed. It returns at once unless the check interval has passed, and
// only one concurrent caller does the check.
func (r *Reloader) Refresh(ctx context.Context) {
	now := time.Now().UnixNano()
	last := r.lastChecked.Load()
	if now-last < r.checkInterval.Nanoseconds() {
		return
	}
	if !r.lastChecked.CompareAndSwap(last, now) {
		return
	}

	if err := r.reload(ctx); err != nil {
		r.logger.WarnWithCtx(ctx, "Failed to reload issuer entry, keeping last good",
			"err", errors.GetErrorStackString(err, -1))
	}
}

func (r *Reloader) reload(ctx context.Context) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// the kubelet swaps the ..data symlink to publish a new version, which makes the path resolve to a
	// different file, so comparing the file identity and its modification time sees every update
	info, err := os.Stat(r.path)
	if err != nil {
		return errors.Wrapf(err, "Failed to stat issuer entry '%s'", r.path)
	}

	if r.checkedInfo != nil && os.SameFile(r.checkedInfo, info) && r.checkedInfo.ModTime().Equal(info.ModTime()) {
		return nil
	}

	content, err := os.ReadFile(r.path)
	if err != nil {
		return errors.Wrapf(err, "Failed to read issuer entry '%s'", r.path)
	}
	r.checkedInfo = info

	config, err := parseConfig(content)
	if err != nil {
		return errors.Wrapf(err, "Invalid issuer entry '%s'", r.path)
	}

	previous := r.current.Load()
	changed := changedFields(previous, config)

	// a new file can hold the same entry, e.g. a ConfigMap re-applied with the same values but different
	// whitespace or key order: `{"issuer_url":"https://idp"}` vs `{ "issuer_url": "https://idp" }`
	if previous != nil && len(changed) == 0 {
		return nil
	}

	if previous == nil {
		r.logger.InfoWithCtx(ctx, "Issuer entry loaded", "fields", changed)
	} else {
		r.logger.InfoWithCtx(ctx, "Issuer entry reloaded", "changedFields", changed)
	}

	r.current.Store(config)
	return nil
}

func parseConfig(content []byte) (*Config, error) {
	config := &Config{}
	if err := json.Unmarshal(content, config); err != nil {
		return nil, errors.Wrap(err, "Failed to parse issuer entry")
	}

	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "Failed to validate issuer entry")
	}

	if config.IdentityClaim == "" {
		config.IdentityClaim = DefaultIdentityClaim
	}

	return config, nil
}

// changedFields maps the JSON name of each field that differs to its new value. A nil previous (the first
// load) is compared as an empty entry, so every field that is set is listed.
func changedFields(previous, current *Config) map[string]interface{} {
	if previous == nil {
		previous = &Config{}
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
