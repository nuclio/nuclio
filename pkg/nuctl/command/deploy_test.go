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

package command

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
)

type deployTestSuite struct {
	suite.Suite
}

func (suite *deployTestSuite) TestParseValidResourceAllocation() {
	var resourceList v1.ResourceList

	err := parseResourceAllocations(stringSliceFlag{"cpu=3", "memory=128M"}, &resourceList)
	suite.Require().NoError(err, "Parse resources should succeed")

	cpuQuantity, valid := resourceList.Cpu().AsInt64()
	suite.Require().True(valid, "CPU quantity must be int64")
	suite.Require().Equal(int64(3), cpuQuantity)

	memoryQuantity, valid := resourceList.Memory().AsInt64()
	suite.Require().True(valid, "Memory quantity must be int64")
	suite.Require().Equal(int64(128000000), memoryQuantity)
}

func (suite *deployTestSuite) TestParseInvalidResourceAllocation() {
	var resourceList v1.ResourceList

	err := parseResourceAllocations(stringSliceFlag{"cpu", "memory=128M"}, &resourceList)
	suite.Require().Error(err, "Parse resources should not succeed")

	err = parseResourceAllocations(stringSliceFlag{"cpu=aaaaaa", "memory=128M"}, &resourceList)
	suite.Require().Error(err, "Parse resources should not succeed")
}

func (suite *deployTestSuite) TestParseValidVolume() {
	var volumesList []functionconfig.Volume

	volumesList, err := parseVolumes(stringSliceFlag{"/path/:/path/", "/path/:/"})
	suite.Require().NoError(err, "Parse volume should succeed")
	suite.Require().NotEmpty(volumesList)
}

func (suite *deployTestSuite) TestParseInvalidVolume() {
	_, err := parseVolumes(stringSliceFlag{"/path/:/path/:", "/path:/path"})
	suite.Require().Error(err, "Parse src is invalid, should not succeed")

	_, err = parseVolumes(stringSliceFlag{"/path/:", "/path:/path"})
	suite.Require().Error(err, "Parse src is invalid, should not succeed")

	_, err = parseVolumes(stringSliceFlag{":", "/path:/path"})
	suite.Require().Error(err, "Parse src is invalid, should not succeed")
}

func TestDeployTestSuite(t *testing.T) {
	suite.Run(t, new(deployTestSuite))
}
