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

package functioncr

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type FunctionTestSuite struct {
	suite.Suite
	function Function
}

func (suite *FunctionTestSuite) TestOnlyName() {
	suite.function.Name = "just_name"

	name, version, err := suite.function.GetNameAndVersion()
	suite.Require().NoError(err)
	suite.Require().Equal("just_name", name)
	suite.Require().Nil(version)
}

func (suite *FunctionTestSuite) TestNameAndVersion() {
	suite.function.Name = "111ValidName123---30"

	name, version, err := suite.function.GetNameAndVersion()
	suite.Require().NoError(err)
	suite.Require().Equal("111ValidName123", name)
	suite.Require().Equal(30, *version)
}

func (suite *FunctionTestSuite) TestInvalidName() {
	suite.function.Name = ""
	_, _, err := suite.function.GetNameAndVersion()
	suite.Require().Error(err)

	suite.function.Name = "@invalidchars"
	_, _, err = suite.function.GetNameAndVersion()
	suite.Require().Error(err)

	suite.function.Name = "valid---invalidversion"
	_, _, err = suite.function.GetNameAndVersion()
	suite.Require().Error(err)
}

func (suite *FunctionTestSuite) TestGetNamespacedName() {
	suite.function.Name = "namepart"
	suite.function.Namespace = "namespacepart"

	namepacedName := suite.function.GetNamespacedName()
	suite.Require().Equal("namespacepart.namepart", namepacedName)
}

func TestFunctionTestSuite(t *testing.T) {
	suite.Run(t, new(FunctionTestSuite))
}
