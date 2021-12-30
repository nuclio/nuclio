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

package inlineparser

import (
	"archive/zip"
	"io/ioutil"
	"os"
	"testing"

	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

var configData = `meta:
  name: java-reverser
spec:
  runtime: java
  handler: nuclio-test-handler-1.0-SNAPSHOT.jar:ReverseEventHandler
  triggers:
    incrementor_http:
      maxWorkers: 1
      kind: http
`

var configFile = "function.yaml"

type JarParserTestSuite struct {
	suite.Suite
}

func (suite *JarParserTestSuite) createJar() string {
	tmpFile, err := ioutil.TempFile("", "nuclio-test-jar")
	suite.Require().NoError(err)

	defer tmpFile.Close()
	zipWrier := zip.NewWriter(tmpFile)
	defer zipWrier.Close()

	out, err := zipWrier.Create(configFile)
	suite.Require().NoError(err)

	n, err := out.Write([]byte(configData))
	suite.Require().NoError(err)
	suite.Require().Equal(len(configData), n)

	return tmpFile.Name()
}

func (suite *JarParserTestSuite) TestJarParser() {
	logger, err := nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	jarPath := suite.createJar()
	defer os.Remove(jarPath) // nolint: errcheck
	parser := NewJarParser(logger)
	config, err := parser.Parse(jarPath)
	suite.Require().NoError(err)

	_, ok := config["configure"]
	suite.Require().True(ok)

	_, ok = config["configure"]["function.yaml"]
	suite.Require().True(ok)
}

func TestJarParserTestSuite(t *testing.T) {
	suite.Run(t, new(JarParserTestSuite))
}
