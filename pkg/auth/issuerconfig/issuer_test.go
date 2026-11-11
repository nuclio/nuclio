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

package issuerconfig

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	issuerWithAudienceA = `{"issuer_url": "https://idp.example.com", "audiences": ["a"]}`
	issuerWithAudienceB = `{"issuer_url": "https://idp.example.com", "audiences": ["b"]}`
)

type ConfigTestSuite struct {
	suite.Suite
	logger logger.Logger
	ctx    context.Context
}

func (suite *ConfigTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("issuer-config-test")
	suite.Require().NoError(err)
	suite.ctx = context.Background()
}

func (suite *ConfigTestSuite) TestParseConfig() {
	for _, testCase := range []struct {
		name                  string
		content               string
		expectedError         string
		expectedTTL           time.Duration
		expectedIdentityClaim string
	}{
		{name: "minimal gets default identity claim", content: `{"issuer_url": "https://idp.example.com"}`, expectedIdentityClaim: "sub"},
		{
			name: "all fields",
			content: `{"issuer_url": "https://idp.example.com", "jwks_url": "https://idp.example.com/keys",
				"audiences": ["a", "b"], "identity_claim": "email", "jwks_cache_ttl": "1h"}`,
			expectedTTL:           time.Hour,
			expectedIdentityClaim: "email",
		},
		{
			name:                  "seconds",
			content:               `{"issuer_url": "https://idp.example.com", "jwks_cache_ttl": "30s"}`,
			expectedTTL:           30 * time.Second,
			expectedIdentityClaim: "sub",
		},
		{
			name:                  "ttl value is not validated",
			content:               `{"issuer_url": "https://idp.example.com", "jwks_cache_ttl": "-1m"}`,
			expectedTTL:           -time.Minute,
			expectedIdentityClaim: "sub",
		},
		{
			name:                  "unknown field is ignored",
			content:               `{"issuer_url": "https://idp.example.com", "extra": 1}`,
			expectedIdentityClaim: "sub",
		},
		{name: "missing issuer url", content: `{}`, expectedError: "Invalid issuer_url"},
		{name: "http issuer url", content: `{"issuer_url": "http://idp.example.com"}`, expectedError: "Invalid issuer_url"},
		{name: "relative issuer url", content: `{"issuer_url": "/realms/test"}`, expectedError: "Invalid issuer_url"},
		{
			name:          "http jwks url",
			content:       `{"issuer_url": "https://idp.example.com", "jwks_url": "http://idp.example.com/keys"}`,
			expectedError: "Invalid jwks_url",
		},
		{
			name:          "ttl is not parsable",
			content:       `{"issuer_url": "https://idp.example.com", "jwks_cache_ttl": "soon"}`,
			expectedError: "Failed to parse issuer entry",
		},
		{
			name:          "ttl is a number",
			content:       `{"issuer_url": "https://idp.example.com", "jwks_cache_ttl": 60}`,
			expectedError: "Failed to parse issuer entry",
		},
		{name: "not json", content: `issuer`, expectedError: "Failed to parse issuer entry"},
	} {
		suite.Run(testCase.name, func() {
			config, err := parseConfig([]byte(testCase.content))
			if testCase.expectedError != "" {
				suite.Require().Error(err)
				suite.Require().Contains(errors.GetErrorStackString(err, -1), testCase.expectedError)
				return
			}
			suite.Require().NoError(err)
			suite.Require().Equal(testCase.expectedTTL, config.JWKSCacheTTL.Duration)
			suite.Require().Equal(testCase.expectedIdentityClaim, config.IdentityClaim)
		})
	}
}

func (suite *ConfigTestSuite) TestChangedFields() {
	base := &Config{
		IssuerURL:     "https://a.example.com",
		JWKSURL:       "https://a.example.com/keys",
		Audiences:     []string{"a"},
		IdentityClaim: "sub",
		JWKSCacheTTL:  metav1.Duration{Duration: time.Minute},
	}

	for _, testCase := range []struct {
		name     string
		previous *Config
		current  *Config
		expected map[string]interface{}
	}{
		{
			name:     "nothing changed",
			previous: base,
			current: &Config{
				IssuerURL:     "https://a.example.com",
				JWKSURL:       "https://a.example.com/keys",
				Audiences:     []string{"a"},
				IdentityClaim: "sub",
				JWKSCacheTTL:  metav1.Duration{Duration: time.Minute},
			},
			expected: map[string]interface{}{},
		},
		{
			name:     "everything changed",
			previous: base,
			current: &Config{
				IssuerURL:     "https://b.example.com",
				JWKSURL:       "https://b.example.com/keys",
				Audiences:     []string{"b"},
				IdentityClaim: "email",
				JWKSCacheTTL:  metav1.Duration{Duration: time.Hour},
			},
			expected: map[string]interface{}{
				"issuer_url":     "https://b.example.com",
				"jwks_url":       "https://b.example.com/keys",
				"audiences":      []string{"b"},
				"identity_claim": "email",
				"jwks_cache_ttl": "1h0m0s",
			},
		},
		{
			name:     "no previous lists the fields that are set",
			previous: nil,
			current:  &Config{IssuerURL: "https://a.example.com", Audiences: []string{"a"}},
			expected: map[string]interface{}{
				"issuer_url": "https://a.example.com",
				"audiences":  []string{"a"},
			},
		},
		{
			name:     "empty and missing audiences are the same",
			previous: &Config{},
			current:  &Config{Audiences: []string{}},
			expected: map[string]interface{}{},
		},
	} {
		suite.Run(testCase.name, func() {
			suite.Require().Equal(testCase.expected, changedFields(testCase.previous, testCase.current))
		})
	}
}

func (suite *ConfigTestSuite) TestNewReloader() {
	for _, testCase := range []struct {
		name              string
		content           *string // nil: the file does not exist
		expectedError     string
		expectedAudiences []string
	}{
		{name: "valid file", content: stringPointer(issuerWithAudienceA), expectedAudiences: []string{"a"}},
		{name: "missing file", expectedError: "Failed to stat issuer entry"},
		{name: "invalid file", content: stringPointer(`{"issuer_url": "http://idp.example.com"}`), expectedError: "Invalid issuer entry"},
	} {
		suite.Run(testCase.name, func() {
			mount := newMountedIssuerFile(suite.T().TempDir())
			if testCase.content != nil {
				mount.publish(suite.Require(), *testCase.content)
			}

			reloader, err := NewReloader(suite.ctx, suite.logger, mount.path(), time.Hour)
			if testCase.expectedError != "" {
				suite.Require().Error(err)
				suite.Require().Contains(errors.GetErrorStackString(err, -1), testCase.expectedError)
				return
			}
			suite.Require().NoError(err)
			suite.Require().Equal(testCase.expectedAudiences, reloader.Get().Audiences)
		})
	}
}

func (suite *ConfigTestSuite) TestRefreshHonorsCheckInterval() {
	for _, testCase := range []struct {
		name              string
		checkInterval     time.Duration
		expectedAudiences []string
	}{
		{name: "interval not passed keeps the loaded entry", checkInterval: time.Hour, expectedAudiences: []string{"a"}},
		{name: "interval passed picks up the update", checkInterval: 0, expectedAudiences: []string{"b"}},
	} {
		suite.Run(testCase.name, func() {
			mount := newMountedIssuerFile(suite.T().TempDir())
			mount.publish(suite.Require(), issuerWithAudienceA)
			reloader, err := NewReloader(suite.ctx, suite.logger, mount.path(), testCase.checkInterval)
			suite.Require().NoError(err)

			mount.publish(suite.Require(), issuerWithAudienceB)
			reloader.Refresh(suite.ctx)

			suite.Require().Equal(testCase.expectedAudiences, reloader.Get().Audiences)
		})
	}
}

func (suite *ConfigTestSuite) TestRefreshAfterConfigMapUpdate() {
	for _, testCase := range []struct {
		name              string
		updates           []string // published one after the other, with a Refresh after each
		expectedAudiences []string
		expectSameEntry   bool // the loaded entry object is not replaced
	}{
		{name: "new version is picked up", updates: []string{issuerWithAudienceB}, expectedAudiences: []string{"b"}},
		{name: "invalid json keeps last good", updates: []string{`{`}, expectedAudiences: []string{"a"}},
		{
			name:              "invalid issuer url keeps last good",
			updates:           []string{`{"issuer_url": "http://idp.example.com", "audiences": ["b"]}`},
			expectedAudiences: []string{"a"},
		},
		{name: "valid update after an invalid one is picked up", updates: []string{`{`, issuerWithAudienceB}, expectedAudiences: []string{"b"}},
		{
			name:              "same entry written differently keeps the entry",
			updates:           []string{`{ "issuer_url": "https://idp.example.com", "audiences": ["a"] }`},
			expectedAudiences: []string{"a"},
			expectSameEntry:   true,
		},
	} {
		suite.Run(testCase.name, func() {
			mount := newMountedIssuerFile(suite.T().TempDir())
			mount.publish(suite.Require(), issuerWithAudienceA)
			reloader, err := NewReloader(suite.ctx, suite.logger, mount.path(), 0)
			suite.Require().NoError(err)
			loaded := reloader.Get()

			for _, update := range testCase.updates {
				mount.publish(suite.Require(), update)
				reloader.Refresh(suite.ctx)
			}

			suite.Require().Equal(testCase.expectedAudiences, reloader.Get().Audiences)
			if testCase.expectSameEntry {
				suite.Require().Same(loaded, reloader.Get())
			}
		})
	}
}

// --- test helpers

// mountedIssuerFile lays the file out like a kubelet-mounted ConfigMap: issuer.json is a symlink to
// ..data/issuer.json, and ..data is a symlink that is swapped to the directory of the newest version.
type mountedIssuerFile struct {
	dir     string
	version int
}

func newMountedIssuerFile(dir string) *mountedIssuerFile {
	return &mountedIssuerFile{dir: dir}
}

func (m *mountedIssuerFile) path() string {
	return filepath.Join(m.dir, "issuer.json")
}

// publish writes content as a new version and swaps ..data to it atomically, as the kubelet does.
func (m *mountedIssuerFile) publish(require *require.Assertions, content string) {
	m.version++
	versionDir := fmt.Sprintf("..version%d", m.version)
	require.NoError(os.Mkdir(filepath.Join(m.dir, versionDir), 0o755))
	require.NoError(os.WriteFile(filepath.Join(m.dir, versionDir, "issuer.json"), []byte(content), 0o644))

	temporaryLink := filepath.Join(m.dir, "..data_tmp")
	require.NoError(os.Symlink(versionDir, temporaryLink))
	require.NoError(os.Rename(temporaryLink, filepath.Join(m.dir, "..data")))

	if m.version == 1 {
		require.NoError(os.Symlink(filepath.Join("..data", "issuer.json"), m.path()))
	}
}

func stringPointer(value string) *string {
	return &value
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}
