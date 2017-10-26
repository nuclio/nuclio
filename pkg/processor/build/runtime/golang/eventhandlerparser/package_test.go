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

package eventhandlerparser

import (
	/*
		"fmt"
		"io/ioutil"
		"os"
		"path/filepath"
		"sort"
		"text/template"

	*/

	"testing"

	"github.com/nuclio/nuclio/test/suite"
)

type PackageParserSuite struct {
	suite.NuclioTestSuite

	parser *PackageHandlerParser
}

func (suite *PackageParserSuite) SetupSuite() {
	suite.NuclioTestSuite.SetupSuite()
	var err error
	suite.parser, err = NewPackageHandlerParser(suite.Logger)
	suite.Require().NoError(err)
}

func (suite *PackageParserSuite) TestParse() {
	packageName := "github.com/nuclio/nuclio-sdk/examples/hello-world"

	packages, handlers, err := suite.parser.ParseEventHandlers(packageName)
	suite.Require().NoError(err)
	suite.Require().Equal(1, len(packages))
	suite.Require().Equal(packageName, packages[0])
	suite.Require().Equal(1, len(handlers))
	suite.Require().Equal("Handler", handlers[0])
}

func TestPackageParser(t *testing.T) {
	suite.Run(t, new(PackageParserSuite))
}
