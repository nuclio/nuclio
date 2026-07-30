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

package containerimagebuilderpusher

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ContainerBuilderConfigurationTestSuite struct {
	suite.Suite
}

func (suite *ContainerBuilderConfigurationTestSuite) TestExistingValuePriorityOverEnv() {
	suite.T().Setenv("NUCLIO_KANIKO_CONTAINER_IMAGE", "from-env:latest")
	suite.T().Setenv("NUCLIO_BUILD_INSECURE_PUSH_REGISTRY", "false")

	existing := &ContainerBuilderConfiguration{
		Kaniko:               KanikoConfig{Image: "from-platform-config:latest"},
		InsecurePushRegistry: true,
		Buildah:              BuildahConfig{RootlessMode: "hostusers", StorageDriver: "overlay", Isolation: "chroot"},
	}

	config, err := NewContainerBuilderConfiguration(existing)
	suite.Require().NoError(err)

	suite.Equal("from-platform-config:latest", config.Kaniko.Image)
	suite.True(config.InsecurePushRegistry)
}

func (suite *ContainerBuilderConfigurationTestSuite) TestUnsetFieldsFallBackToEnv() {
	suite.T().Setenv("NUCLIO_KANIKO_CONTAINER_IMAGE", "from-env:latest")
	suite.T().Setenv("NUCLIO_BUILD_INSECURE_PUSH_REGISTRY", "true")

	existing := &ContainerBuilderConfiguration{}

	config, err := NewContainerBuilderConfiguration(existing)
	suite.Require().NoError(err)

	suite.Equal("from-env:latest", config.Kaniko.Image)
	suite.True(config.InsecurePushRegistry)
}

func (suite *ContainerBuilderConfigurationTestSuite) TestNilExistingBehavesEnvOnly() {
	suite.T().Setenv("NUCLIO_KANIKO_CONTAINER_IMAGE", "from-env:latest")

	config, err := NewContainerBuilderConfiguration(nil)
	suite.Require().NoError(err)

	suite.Equal("from-env:latest", config.Kaniko.Image)
}

func (suite *ContainerBuilderConfigurationTestSuite) TestSecretNamesMergeSingularAndList() {
	suite.T().Setenv("NUCLIO_REGISTRY_CREDENTIALS_SECRET_NAMES", "from-env")

	existing := &ContainerBuilderConfiguration{
		DefaultRegistryCredentialsSecretName:  "singular",
		DefaultRegistryCredentialsSecretNames: []string{"from-list"},
	}

	config, err := NewContainerBuilderConfiguration(existing)
	suite.Require().NoError(err)

	suite.Equal([]string{"singular", "from-list", "from-env"}, config.DefaultRegistryCredentialsSecretNames)
}

func (suite *ContainerBuilderConfigurationTestSuite) TestPythonImageDefaultsToIguazioPython() {
	config, err := NewContainerBuilderConfiguration(nil)
	suite.Require().NoError(err)

	suite.Equal("gcr.io/iguazio/python:3.11", config.PythonImage)
}

func (suite *ContainerBuilderConfigurationTestSuite) TestPythonImageHonorsEnv() {
	suite.T().Setenv("NUCLIO_PYTHON_BASE_IMAGE_NAME", "custom-image:latest")

	config, err := NewContainerBuilderConfiguration(nil)
	suite.Require().NoError(err)

	suite.Equal("custom-image:latest", config.PythonImage)
}

func (suite *ContainerBuilderConfigurationTestSuite) TestPythonImagePullPolicyDefaultsToIfNotPresent() {
	config, err := NewContainerBuilderConfiguration(nil)
	suite.Require().NoError(err)

	suite.Equal("IfNotPresent", config.PythonImagePullPolicy)
}

func (suite *ContainerBuilderConfigurationTestSuite) TestPythonImagePullPolicyHonorsEnv() {
	suite.T().Setenv("NUCLIO_PYTHON_BASE_IMAGE_PULL_POLICY", "Always")

	config, err := NewContainerBuilderConfiguration(nil)
	suite.Require().NoError(err)

	suite.Equal("Always", config.PythonImagePullPolicy)
}

func TestContainerBuilderConfigurationTestSuite(t *testing.T) {
	suite.Run(t, new(ContainerBuilderConfigurationTestSuite))
}
