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
	"context"
	"regexp"
	"testing"

	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher/registryhelpers"
	"github.com/nuclio/nuclio/pkg/platform/kube/clients/kube"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

var (
	budTLSVerifyRegexp  = regexp.MustCompile(`buildah bud[^\n]*--tls-verify=false`)
	pushTLSVerifyRegexp = regexp.MustCompile(`buildah push[^\n]*--tls-verify=false`)
)

type BuildahTestSuite struct {
	suite.Suite
	logger  logger.Logger
	buildah *Buildah
}

func (suite *BuildahTestSuite) SetupTest() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.buildah = &Buildah{
		jobRunner: &jobRunner{
			builderName:   BuildahKind,
			logger:        suite.logger,
			kubeClientSet: kube.NewClientWithRetryFromClient(k8sfake.NewClientset()),
			builderConfiguration: &ContainerBuilderConfiguration{
				BusyBoxImage: "busybox:stable",
				Buildah: BuildahConfig{
					Image:           "quay.io/buildah/stable:v1.43.1",
					ImagePullPolicy: "IfNotPresent",
					RootlessMode:    "caps",
					StorageDriver:   "overlay",
					Isolation:       "chroot",
				},
			},
		},
	}
}

func (suite *BuildahTestSuite) TestCompileBuildahContainerTLSVerifyFlags() {
	for _, testCase := range []struct {
		name                 string
		insecurePullRegistry bool
		insecurePushRegistry bool
		expectPullTLSVerify  bool
		expectPushTLSVerify  bool
	}{
		{name: "SecurePullSecurePush"},
		{
			name:                 "InsecurePullSecurePush",
			insecurePullRegistry: true,
			expectPullTLSVerify:  true,
		},
		{
			name:                 "SecurePullInsecurePush",
			insecurePushRegistry: true,
			expectPushTLSVerify:  true,
		},
		{
			name:                 "InsecurePullInsecurePush",
			insecurePullRegistry: true,
			insecurePushRegistry: true,
			expectPullTLSVerify:  true,
			expectPushTLSVerify:  true,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.buildah.builderConfiguration.InsecurePullRegistry = testCase.insecurePullRegistry
			suite.buildah.builderConfiguration.InsecurePushRegistry = testCase.insecurePushRegistry

			container := suite.buildah.compileBuildahContainer(suite.newBuildOptions())

			suite.Require().Len(container.Args, 2)
			command := container.Args[1]

			budTLSVerify := budTLSVerifyRegexp.MatchString(command)
			pushTLSVerify := pushTLSVerifyRegexp.MatchString(command)

			suite.Equal(testCase.expectPullTLSVerify, budTLSVerify)
			suite.Equal(testCase.expectPushTLSVerify, pushTLSVerify)
		})
	}
}

func (suite *BuildahTestSuite) TestCompileJobSpecRootlessMode() {
	for _, testCase := range []struct {
		name               string
		rootlessMode       string
		expectHostUsers    bool
		expectCapabilities bool
	}{
		{name: "Caps", rootlessMode: "caps", expectCapabilities: true},
		{name: "Hostusers", rootlessMode: "hostusers", expectHostUsers: true},
	} {
		suite.Run(testCase.name, func() {
			suite.buildah.builderConfiguration.Buildah.RootlessMode = testCase.rootlessMode

			jobSpec, err := suite.buildah.compileJobSpec(context.Background(), "default", suite.newBuildOptions(), "bundle.tar")
			suite.Require().NoError(err)

			podSpec := jobSpec.Spec.Template.Spec

			if testCase.expectHostUsers {
				suite.Require().NotNil(podSpec.HostUsers)
				suite.False(*podSpec.HostUsers)
				suite.Nil(podSpec.Containers[0].SecurityContext)
			} else {
				suite.Nil(podSpec.HostUsers)
			}

			if testCase.expectCapabilities {
				suite.Require().NotNil(podSpec.Containers[0].SecurityContext)
				suite.Contains(podSpec.Containers[0].SecurityContext.Capabilities.Add, v1.Capability("SETUID"))
				suite.Contains(podSpec.Containers[0].SecurityContext.Capabilities.Add, v1.Capability("SETGID"))
				suite.True(*podSpec.Containers[0].SecurityContext.AllowPrivilegeEscalation)
			}
		})
	}
}

func (suite *BuildahTestSuite) TestCompileJobSpecNoAuthVolumeWithoutSecret() {
	jobSpec, err := suite.buildah.compileJobSpec(context.Background(), "default", suite.newBuildOptions(), "bundle.tar")
	suite.Require().NoError(err)

	podSpec := jobSpec.Spec.Template.Spec
	suite.Len(podSpec.Volumes, 1)
	suite.Equal("tmp", podSpec.Volumes[0].Name)
	suite.Len(podSpec.Containers[0].VolumeMounts, 1)
}

func (suite *BuildahTestSuite) TestCompileJobSpecAuthVolumeWithSecret() {
	buildOptions := suite.newBuildOptions()
	buildOptions.SecretName = "my-registry-secret"

	jobSpec, err := suite.buildah.compileJobSpec(context.Background(), "default", buildOptions, "bundle.tar")
	suite.Require().NoError(err)

	podSpec := jobSpec.Spec.Template.Spec
	suite.Len(podSpec.Volumes, 2)
	suite.Require().Len(podSpec.Containers[0].VolumeMounts, 2)

	authMount := podSpec.Containers[0].VolumeMounts[1]
	suite.Equal(registryhelpers.AuthVolumeName, authMount.Name)
	suite.True(authMount.ReadOnly, "auth secret mount must be read-only")
}

func (suite *BuildahTestSuite) TestCompileJobSpecCloudAuthLoginContainersRunBeforeMerge() {
	suite.buildah.builderConfiguration.PythonImage = "python:test"

	buildOptions := suite.newBuildOptions()
	buildOptions.RegistryURL = "myregistry.azurecr.io"

	jobSpec, err := suite.buildah.compileJobSpec(context.Background(), "default", buildOptions, "bundle.tar")
	suite.Require().NoError(err)

	podSpec := jobSpec.Spec.Template.Spec
	suite.Require().Len(podSpec.InitContainers, 4)
	loginContainer := podSpec.InitContainers[2]
	mergeContainer := podSpec.InitContainers[3]
	suite.Contains(loginContainer.Name, "registry-login-azure")
	suite.Equal("merge-authfile", mergeContainer.Name)

	// the login container writes its token into the dir the merge container reads it from
	tokenDir := registryhelpers.TokenDirVolumeMount().MountPath
	suite.Contains(loginContainer.Args[1], tokenDir)
	suite.Contains(mergeContainer.Args, "--cloud-tokens="+tokenDir)
}

func (suite *BuildahTestSuite) TestNewContainerBuilderConfigurationValidatesRootlessMode() {
	for _, testCase := range []struct {
		name      string
		envValue  string
		expected  string
		expectErr bool
	}{
		{name: "Unset", envValue: "", expected: "caps"},
		{name: "Caps", envValue: "caps", expected: "caps"},
		{name: "Hostusers", envValue: "hostusers", expected: "hostusers"},
		{name: "Invalid", envValue: "rootful", expectErr: true},
	} {
		suite.Run(testCase.name, func() {
			if testCase.envValue != "" {
				suite.T().Setenv("NUCLIO_BUILDAH_ROOTLESS_MODE", testCase.envValue)
			}

			config, err := NewContainerBuilderConfiguration(nil)
			if testCase.expectErr {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)
			suite.Equal(testCase.expected, config.Buildah.RootlessMode)
		})
	}
}

func (suite *BuildahTestSuite) TestNewContainerBuilderConfigurationValidatesStorageDriver() {
	for _, testCase := range []struct {
		name      string
		envValue  string
		expected  string
		expectErr bool
	}{
		{name: "Unset", envValue: "", expected: "overlay"},
		{name: "Overlay", envValue: "overlay", expected: "overlay"},
		{name: "VFS", envValue: "vfs", expected: "vfs"},
		{name: "Invalid", envValue: "btrfs", expectErr: true},
	} {
		suite.Run(testCase.name, func() {
			if testCase.envValue != "" {
				suite.T().Setenv("NUCLIO_BUILDAH_STORAGE_DRIVER", testCase.envValue)
			}

			config, err := NewContainerBuilderConfiguration(nil)
			if testCase.expectErr {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)
			suite.Equal(testCase.expected, config.Buildah.StorageDriver)
		})
	}
}

func (suite *BuildahTestSuite) TestNewContainerBuilderConfigurationValidatesIsolation() {
	for _, testCase := range []struct {
		name      string
		envValue  string
		expected  string
		expectErr bool
	}{
		{name: "Unset", envValue: "", expected: "chroot"},
		{name: "Chroot", envValue: "chroot", expected: "chroot"},
		{name: "OCI", envValue: "oci", expected: "oci"},
		{name: "Invalid", envValue: "rootless", expectErr: true},
	} {
		suite.Run(testCase.name, func() {
			if testCase.envValue != "" {
				suite.T().Setenv("NUCLIO_BUILDAH_ISOLATION", testCase.envValue)
			}

			config, err := NewContainerBuilderConfiguration(nil)
			if testCase.expectErr {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)
			suite.Equal(testCase.expected, config.Buildah.Isolation)
		})
	}
}

func (suite *BuildahTestSuite) newBuildOptions() *BuildOptions {
	return &BuildOptions{
		Image:       "my-func:latest",
		ContextDir:  "/some/context",
		RegistryURL: "registry.example.com",
		DockerfileInfo: &runtime.ProcessorDockerfileInfo{
			DockerfilePath: "/some/context/Dockerfile",
		},
	}
}

func TestBuildahTestSuite(t *testing.T) {
	suite.Run(t, new(BuildahTestSuite))
}
