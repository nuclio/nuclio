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
	"testing"
	"time"

	"github.com/nuclio/errors"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type IssuerConfigTestSuite struct {
	suite.Suite
}

func (suite *IssuerConfigTestSuite) TestParseIssuerConfig() {
	for _, testCase := range []struct {
		name          string
		content       string
		expectedError string
		expectedTTL   time.Duration
	}{
		{name: "minimal", content: `{"issuer_url": "https://idp.example.com"}`},
		{
			name: "all fields",
			content: `{"issuer_url": "https://idp.example.com", "jwks_url": "https://idp.example.com/keys",
				"audiences": ["a", "b"], "identity_claim": "email", "jwks_cache_ttl": "1h"}`,
			expectedTTL: time.Hour,
		},
		{name: "seconds", content: `{"issuer_url": "https://idp.example.com", "jwks_cache_ttl": "30s"}`, expectedTTL: 30 * time.Second},
		{name: "ttl value is not validated", content: `{"issuer_url": "https://idp.example.com", "jwks_cache_ttl": "-1m"}`, expectedTTL: -time.Minute},
		{name: "unknown field is ignored", content: `{"issuer_url": "https://idp.example.com", "extra": 1}`},
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
			issuerConfig, err := parseIssuerConfig([]byte(testCase.content))
			if testCase.expectedError != "" {
				suite.Require().Error(err)
				suite.Require().Contains(errors.GetErrorStackString(err, -1), testCase.expectedError)
				return
			}
			suite.Require().NoError(err)
			suite.Require().Equal(testCase.expectedTTL, issuerConfig.JWKSCacheTTL.Duration)
		})
	}
}

func (suite *IssuerConfigTestSuite) TestChangedFields() {
	base := &IssuerConfig{
		IssuerURL:     "https://a.example.com",
		JWKSURL:       "https://a.example.com/keys",
		Audiences:     []string{"a"},
		IdentityClaim: "sub",
		JWKSCacheTTL:  metav1.Duration{Duration: time.Minute},
	}

	for _, testCase := range []struct {
		name     string
		previous *IssuerConfig
		current  *IssuerConfig
		expected map[string]interface{}
	}{
		{
			name:     "nothing changed",
			previous: base,
			current: &IssuerConfig{
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
			current: &IssuerConfig{
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
			current:  &IssuerConfig{IssuerURL: "https://a.example.com", Audiences: []string{"a"}},
			expected: map[string]interface{}{
				"issuer_url": "https://a.example.com",
				"audiences":  []string{"a"},
			},
		},
		{
			name:     "empty and missing audiences are the same",
			previous: &IssuerConfig{},
			current:  &IssuerConfig{Audiences: []string{}},
			expected: map[string]interface{}{},
		},
	} {
		suite.Run(testCase.name, func() {
			suite.Require().Equal(testCase.expected, changedFields(testCase.previous, testCase.current))
		})
	}
}

func TestIssuerConfigTestSuite(t *testing.T) {
	suite.Run(t, new(IssuerConfigTestSuite))
}
