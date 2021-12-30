//go:build test_unit

/*
Copyright 2017 The Nuclio Authors.

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

package dashboard

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/dockercreds"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type DashboardServerTestSuite struct {
	suite.Suite
	Server
	Logger logger.Logger
}

func (suite *DashboardServerTestSuite) SetupTest() {
	var err error
	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)
}

func (suite *DashboardServerTestSuite) TestResolveRegistryURLFromDockerCredentials() {
	dummyUsername := "dummy-user"
	for _, testCase := range []struct {
		credentials             dockercreds.Credentials
		expectedRegistryURLHost string
		Match                   bool
	}{
		{
			credentials:             dockercreds.Credentials{URL: "https://index.docker.io/v1/", Username: dummyUsername},
			expectedRegistryURLHost: "index.docker.io",
		},
		{
			credentials:             dockercreds.Credentials{URL: "index.docker.io/v1/", Username: dummyUsername},
			expectedRegistryURLHost: "index.docker.io",
		},
		{
			credentials:             dockercreds.Credentials{URL: "https://index.docker.io", Username: dummyUsername},
			expectedRegistryURLHost: "index.docker.io",
		},
		{
			credentials:             dockercreds.Credentials{URL: "index.docker.io", Username: dummyUsername},
			expectedRegistryURLHost: "index.docker.io",
		},
	} {
		expectedRegistryURL := testCase.expectedRegistryURLHost + "/" + dummyUsername
		suite.Require().Equal(expectedRegistryURL, suite.resolveDockerCredentialsRegistryURL(testCase.credentials))
	}
}

func TestDashboardServerTestSuite(t *testing.T) {
	suite.Run(t, new(DashboardServerTestSuite))
}
