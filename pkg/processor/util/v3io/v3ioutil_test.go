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

package v3ioutil

import (
	"testing"

	"github.com/nuclio/logger"
	"github.com/stretchr/testify/suite"
)

type v3ioUtilTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *v3ioUtilTestSuite) TestParseURLNoContainerAlias() {
	_, _, _, err := ParseURL("http://host:port/")
	suite.Require().Error(err)
}

func (suite *v3ioUtilTestSuite) TestParseURLNoPath() {
	addr, containerAlias, path, err := ParseURL("http://host:port/containerAlias")
	suite.Require().NoError(err)

	suite.Require().Equal("host:port", addr)
	suite.Require().Equal("containerAlias", containerAlias)
	suite.Require().Equal("", path)
}

func (suite *v3ioUtilTestSuite) TestParseURLWithPath() {
	addr, containerAlias, path, err := ParseURL("http://host:port/containerAlias/path1/path2/path3/")
	suite.Require().NoError(err)

	suite.Require().Equal("host:port", addr)
	suite.Require().Equal("containerAlias", containerAlias)
	suite.Require().Equal("path1/path2/path3", path)
}

func TestCmdRunnerTestSuite(t *testing.T) {
	suite.Run(t, new(v3ioUtilTestSuite))
}
