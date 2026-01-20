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

package serviceaccounttoken

import (
	"os"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/auth/utils"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/suite"
)

const TestSecretKey = "test-secret-key"

type ServiceAccountTokenClientTestSuite struct {
	suite.Suite
	tokenFile *os.File
}

func (suite *ServiceAccountTokenClientTestSuite) SetupTest() {
	var err error
	suite.tokenFile, err = os.CreateTemp("", "sa-token-*")
	suite.Require().NoError(err)
}

func (suite *ServiceAccountTokenClientTestSuite) TearDownTest() {
	if suite.tokenFile != nil {
		_ = suite.tokenFile.Close()
		_ = os.Remove(suite.tokenFile.Name())
	}
}

func (suite *ServiceAccountTokenClientTestSuite) TestGetSATokenSuccess() {
	token := "test-token"
	suite.writeToken(token)
	client, err := NewClient(&platformconfig.ServiceAccountConfig{
		TokenPath: &[]string{suite.tokenFile.Name()}[0],
	})
	suite.Require().NoError(err)
	readToken, err := client.GetSAToken()
	suite.Require().NoError(err)
	suite.Require().Equal(token, readToken)
}

func (suite *ServiceAccountTokenClientTestSuite) TestGetSATokenFileNotFound() {
	badPath := suite.tokenFile.Name() + "-notfound"
	client, err := NewClient(&platformconfig.ServiceAccountConfig{
		TokenPath: &badPath,
	})
	suite.Require().NoError(err)
	_, err = client.GetSAToken()
	suite.Require().Error(err)
}

func (suite *ServiceAccountTokenClientTestSuite) TestTokenCachingAndExpiration() {
	token := utils.GenerateTestJWT(jwt.MapClaims{}, time.Now().Add(2*time.Hour), TestSecretKey)
	suite.writeToken(token)
	client, err := NewClient(&platformconfig.ServiceAccountConfig{
		TokenPath:              &[]string{suite.tokenFile.Name()}[0],
		TokenExpirationSeconds: &[]int{3600}[0],
		TokenRefreshRatio:      &[]float64{0.5}[0],
	})
	suite.Require().NoError(err)
	readToken, err := client.GetSAToken()
	suite.Require().NoError(err)
	suite.Require().Equal(token, readToken)
	// Should use cache
	readToken2, err := client.GetSAToken()
	suite.Require().NoError(err)
	suite.Require().Equal(token, readToken2)
}

func (suite *ServiceAccountTokenClientTestSuite) TestAuthHeaders() {
	token := "test-token"
	suite.writeToken(token)
	client, err := NewClient(&platformconfig.ServiceAccountConfig{
		TokenPath: &[]string{suite.tokenFile.Name()}[0],
	})
	suite.Require().NoError(err)
	authHeaders, err := client.AuthHeaders()
	suite.Require().NoError(err)
	suite.Require().Equal("Bearer "+token, authHeaders[headers.AuthorizationHeader])
}

func (suite *ServiceAccountTokenClientTestSuite) TestEscalateAuthHeaders() {
	token := "test-token"
	suite.writeToken(token)
	client, err := NewClient(&platformconfig.ServiceAccountConfig{
		TokenPath: &[]string{suite.tokenFile.Name()}[0],
	})
	suite.Require().NoError(err)
	authHeaders := map[string]string{"foo": "bar"}
	suite.Require().NoError(client.EscalateAuthHeaders(authHeaders))
	suite.Require().Equal("Bearer "+token, authHeaders[headers.AuthorizationHeader])
	suite.Require().Equal("bar", authHeaders["foo"])
}

func (suite *ServiceAccountTokenClientTestSuite) writeToken(token string) {
	suite.Require().NoError(os.WriteFile(suite.tokenFile.Name(), []byte(token), 0600))
}

func TestServiceAccountTokenClientTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceAccountTokenClientTestSuite))
}
