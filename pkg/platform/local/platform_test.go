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

package local

import (
	"context"
	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"testing"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	mockplatform "github.com/nuclio/nuclio/pkg/platform/mock"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type localPlatformTestSuite struct {
	suite.Suite
	abstractPlatform *abstract.Platform
	logger           logger.Logger
	mockedPlatform   *mockplatform.Platform
	cmdRunner        *cmdrunner.MockRunner
	dockerClient     *dockerclient.MockDockerClient
	platform         *Platform
	ctx              context.Context
}

func (suite *localPlatformTestSuite) SetupSuite() {
	var err error
	common.SetVersionFromEnv()
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err, "Logger should create successfully")
	suite.ctx = context.Background()
}

func (suite *localPlatformTestSuite) SetupTest() {
	var err error
	platformConfig := &platformconfig.Config{}
	suite.mockedPlatform = &mockplatform.Platform{}
	abstractPlatform, err := abstract.NewPlatform(suite.logger, suite.mockedPlatform, platformConfig, "")
	suite.Require().NoError(err, "Could not create platform")

	suite.abstractPlatform = abstractPlatform
	suite.cmdRunner = cmdrunner.NewMockRunner()
	suite.dockerClient = dockerclient.NewMockDockerClient()
	suite.platform, err = NewPlatform(suite.ctx, suite.logger, platformConfig, "")
	suite.Require().NoError(err, "Could not create platform")
	suite.platform.cmdRunner = suite.cmdRunner
	suite.platform.dockerClient = suite.dockerClient
}

func (suite *localPlatformTestSuite) TearDownTest() {
	suite.cmdRunner.AssertExpectations(suite.T())
	suite.dockerClient.AssertExpectations(suite.T())
}

func (suite *localPlatformTestSuite) TestResolveFunctionSpecRequestCPUs() {
	for _, testCase := range []struct {
		name         string
		cpus         string
		expectedCPUs string
	}{
		{
			name:         "CPU - empty",
			cpus:         "",
			expectedCPUs: "",
		},
		{
			name:         "CPU - integer",
			cpus:         "1",
			expectedCPUs: "1.0",
		},
		{
			name:         "CPU - float",
			cpus:         "1.5",
			expectedCPUs: "1.5",
		},
		{
			name:         "CPU - float 2",
			cpus:         ".5",
			expectedCPUs: "0.5",
		},
		{
			// support up to 5 digits after the decimal point
			name:         "CPU - float 3",
			cpus:         "3.14159265",
			expectedCPUs: "3.141593",
		},
	} {
		suite.Run(testCase.name, func() {
			resourceLimits := v1.ResourceList{}
			if testCase.cpus != "" {
				quantity, err := resource.ParseQuantity(testCase.cpus)
				suite.Require().NoError(err, "Could not parse quantity")
				resourceLimits[v1.ResourceCPU] = quantity
			}
			cpus := suite.platform.resolveFunctionSpecRequestCPUs(functionconfig.Spec{
				Resources: v1.ResourceRequirements{
					Limits: resourceLimits,
				},
			})
			suite.Require().Equal(testCase.expectedCPUs, cpus)
		})
	}
}

func (suite *localPlatformTestSuite) TestResolveFunctionSpecRequestMemory() {
	for _, testCase := range []struct {
		name           string
		memory         string
		expectedMemory string
	}{
		{
			name:           "Memory - empty",
			memory:         "",
			expectedMemory: "",
		},
		{
			name:           "Memory - 1Gi",
			memory:         "1Gi",
			expectedMemory: "1073741824b",
		},
		{
			name:           "Memory - 1G",
			memory:         "1G",
			expectedMemory: "1000000000b",
		},
	} {
		suite.Run(testCase.name, func() {
			resourceLimits := v1.ResourceList{}
			if testCase.memory != "" {
				quantity, err := resource.ParseQuantity(testCase.memory)
				suite.Require().NoError(err, "Could not parse quantity")
				resourceLimits[v1.ResourceMemory] = quantity
			}
			cpus := suite.platform.resolveFunctionSpecRequestMemory(functionconfig.Spec{
				Resources: v1.ResourceRequirements{
					Limits: resourceLimits,
				},
			})
			suite.Require().Equal(testCase.expectedMemory, cpus)
		})
	}
}

func TestKubePlatformTestSuite(t *testing.T) {
	suite.Run(t, new(localPlatformTestSuite))
}
