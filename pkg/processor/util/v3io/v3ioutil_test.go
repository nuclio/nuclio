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

package v3ioutil

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

const DefaultPort = 12345

type v3ioUtilTestSuite struct {
	suite.Suite
}

func (suite *v3ioUtilTestSuite) TestParseURLNoContainerAlias() {
	_, _, _, err := ParseURL(fmt.Sprintf("http://host.com:%d/", DefaultPort))
	suite.Require().Error(err)
}

func (suite *v3ioUtilTestSuite) TestParseURLNoPath() {
	addr, containerAlias, path, err := ParseURL(fmt.Sprintf("http://host.com:%d/containerAlias", DefaultPort))
	suite.Require().NoError(err)

	suite.Require().Equal(fmt.Sprintf("host.com:%d", DefaultPort), addr)
	suite.Require().Equal("containerAlias", containerAlias)
	suite.Require().Equal("", path)
}

func (suite *v3ioUtilTestSuite) TestParseURLWithPath() {
	addr, containerAlias, path, err := ParseURL(fmt.Sprintf("http://host.com:%d/containerAlias/path1/path2/path3/", DefaultPort))
	suite.Require().NoError(err)

	suite.Require().Equal(fmt.Sprintf("host.com:%d", DefaultPort), addr)
	suite.Require().Equal("containerAlias", containerAlias)
	suite.Require().Equal("path1/path2/path3", path)
}

func TestCmdRunnerTestSuite(t *testing.T) {
	suite.Run(t, new(v3ioUtilTestSuite))
}
