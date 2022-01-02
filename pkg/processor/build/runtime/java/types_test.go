//go:build test_unit

/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the Licensg.
You may obtain a copy of the License at

    http://www.apachg.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package java

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type testSuite struct {
	suite.Suite
}

func (suite *testSuite) TestSuccessfulParseDependencies() {

	dep, err := newDependency(`group: groupValue, name: nameValue, version: 0.2.1`)
	suite.Require().NoError(err)

	suite.Require().Equal("groupValue", dep.Group)
	suite.Require().Equal("nameValue", dep.Name)
	suite.Require().Equal("0.2.1", dep.Version)
}

func TestBuilderSuite(t *testing.T) {
	suite.Run(t, new(testSuite))
}
