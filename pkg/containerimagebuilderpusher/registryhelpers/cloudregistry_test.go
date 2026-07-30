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

package registryhelpers

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type CloudRegistryTestSuite struct {
	suite.Suite
}

func (suite *CloudRegistryTestSuite) TestHelperForPicksTheRightVendor() {
	for _, testCase := range []struct {
		name     string
		host     string
		expected string
	}{
		{name: "ACR", host: "myregistry.azurecr.io", expected: "*registryhelpers.azureHelper"},
		{name: "GAR", host: "us-central1-docker.pkg.dev", expected: "*registryhelpers.gcpHelper"},
		{name: "ECR", host: "123456789012.dkr.ecr.us-east-1.amazonaws.com", expected: "*registryhelpers.awsHelper"},
		{name: "PlainDockerHub", host: "index.docker.io", expected: ""},
		{name: "Empty", host: "", expected: ""},
	} {
		suite.Run(testCase.name, func() {
			helper := helperFor(testCase.host)
			if testCase.expected == "" {
				suite.Nil(helper)
				return
			}
			suite.Require().NotNil(helper)
			suite.Equal(testCase.expected, fmt.Sprintf("%T", helper))
		})
	}
}

func (suite *CloudRegistryTestSuite) TestNeedsCloudLogin() {
	suite.True(NeedsCloudLogin([]string{"index.docker.io", "myregistry.azurecr.io"}))
	suite.False(NeedsCloudLogin([]string{"index.docker.io", "ghcr.io"}))
	suite.False(NeedsCloudLogin(nil))
	suite.False(NeedsCloudLogin([]string{}))
}

func (suite *CloudRegistryTestSuite) TestBuildLoginContainersSharesOneProviderSecretVolume() {
	hosts := []string{"myregistry.azurecr.io", "us-central1-docker.pkg.dev"}
	cfg := AuthConfig{RegistryProviderSecretName: "registry-provider-creds"}

	containers, volumes, err := BuildLoginContainers(hosts, "", "", cfg, "IfNotPresent")
	suite.Require().NoError(err)

	suite.Require().Len(volumes, 1)
	suite.Equal("registry-provider-creds", volumes[0].Name)

	suite.Require().Len(containers, 2)
	for _, container := range containers {
		found := false
		for _, mount := range container.VolumeMounts {
			if mount.Name == "registry-provider-creds" {
				found = true
			}
		}
		suite.True(found, "container %s missing provider secret mount", container.Name)
	}
}

func TestCloudRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(CloudRegistryTestSuite))
}
