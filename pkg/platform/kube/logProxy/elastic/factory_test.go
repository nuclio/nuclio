//go:build test_unit

/*
Copyright 2025 The Nuclio Authors.

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

package elastic

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/stretchr/testify/suite"
)

type FactoryTestSuite struct {
	suite.Suite
	server *httptest.Server
}

// SetupTest starts a fresh TLS test server (self-signed cert) that mimics the
// search engine's version endpoint for every test.
func (suite *FactoryTestSuite) SetupTest() {
	suite.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":{"number":"8.11.0"}}`))
	}))
}

func (suite *FactoryTestSuite) TearDownTest() {
	suite.server.Close()
}

// TestSSLVerificationModeNoneSkipsCertVerification asserts that setting
// SSLVerificationMode to "none" allows the auto-detection probe to succeed
// against a server presenting a self-signed/untrusted certificate.
func (suite *FactoryTestSuite) TestSSLVerificationModeNoneSkipsCertVerification() {
	config := &platformconfig.ElasticSearchConfig{
		URL:                 suite.server.URL,
		SSLVerificationMode: "none",
	}

	versionInfoInstance, err := getVersionFromSearchEngineWithRetries(config, 1, time.Millisecond, 5*time.Second)
	suite.Require().NoError(err)
	suite.Require().NotNil(versionInfoInstance)
	suite.Equal("8.11.0", versionInfoInstance.Version.Number)
}

// TestDefaultSSLVerificationModeRejectsUntrustedCert asserts that, without
// explicitly disabling SSL verification, the auto-detection probe still
// performs full certificate verification and fails against a self-signed cert.
func (suite *FactoryTestSuite) TestDefaultSSLVerificationModeRejectsUntrustedCert() {
	config := &platformconfig.ElasticSearchConfig{
		URL: suite.server.URL,
	}

	versionInfoInstance, err := getVersionFromSearchEngineWithRetries(config, 1, time.Millisecond, 5*time.Second)
	suite.Require().Error(err)
	suite.Require().Nil(versionInfoInstance)
	suite.Contains(err.Error(), "Failed to connect to search engine endpoint")
}

func TestFactoryTestSuite(t *testing.T) {
	suite.Run(t, new(FactoryTestSuite))
}
