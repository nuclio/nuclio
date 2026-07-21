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

type BuildahTestSuite struct {
	suite.Suite
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

			config, err := NewContainerBuilderConfiguration()
			if testCase.expectErr {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)
			suite.Equal(testCase.expected, config.BuildahRootlessMode)
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

			config, err := NewContainerBuilderConfiguration()
			if testCase.expectErr {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)
			suite.Equal(testCase.expected, config.BuildahStorageDriver)
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

			config, err := NewContainerBuilderConfiguration()
			if testCase.expectErr {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)
			suite.Equal(testCase.expected, config.BuildahIsolation)
		})
	}
}

func TestBuildahTestSuite(t *testing.T) {
	suite.Run(t, new(BuildahTestSuite))
}
