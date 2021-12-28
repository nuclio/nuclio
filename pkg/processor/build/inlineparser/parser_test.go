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
	"io/ioutil"
	"os"
	"testing"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type InlineParserTestSuite struct {
	suite.Suite
	logger logger.Logger
	parser *InlineParser
}

func (suite *InlineParserTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.parser = NewParser(suite.logger, "#")
}

func (suite *InlineParserTestSuite) TestValidBlockSingleChar() {
	content := `
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#      @nuclio.configure
#
#   function.yaml:
#     spec:
#       runtime: "python"
#       handler: parser:handler
#

import simplejson

def handler(context, event):
    """Return a field from within a json"""

    context.logger.info('Hello from Python')
    body = simplejson.loads(event.body.decode('utf-8'))
    return body['return_this']
`
	tmpFile, err := ioutil.TempFile("", "nuclio-parser-test")
	suite.Require().NoError(err)
	suite.Require().NoError(tmpFile.Close())

	defer os.Remove(tmpFile.Name()) // nolint: errcheck

	err = ioutil.WriteFile(tmpFile.Name(), []byte(content), 0600)
	suite.Require().NoError(err)

	blocks, err := suite.parser.Parse(tmpFile.Name())
	suite.Require().NoError(err)

	// get function YAML
	suite.Require().NotEmpty(blocks["configure"])
}

func (suite *InlineParserTestSuite) TestValidBlockSingleCharNoSpaces() {
	content := `
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#@nuclio.configure
#
#function.yaml:
#  spec:
#    runtime: "python"
#    handler: parser:handler
#

import simplejson

def handler(context, event):
    """Return a field from within a json"""

    context.logger.info('Hello from Python')
    body = simplejson.loads(event.body.decode('utf-8'))
    return body['return_this']
`
	tmpFile, err := ioutil.TempFile("", "nuclio-parser-test")
	suite.Require().NoError(err)
	suite.Require().NoError(tmpFile.Close())

	defer os.Remove(tmpFile.Name()) // nolint: errcheck

	err = ioutil.WriteFile(tmpFile.Name(), []byte(content), 0600)
	suite.Require().NoError(err)

	blocks, err := suite.parser.Parse(tmpFile.Name())
	suite.Require().NoError(err)

	// get function YAML
	suite.Require().NotEmpty(blocks["configure"])
}

func (suite *InlineParserTestSuite) TestBlockWithError() {
	content := `
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# @nuclio.configure
# 
# function.yaml:
#   spec:
#     runtime: "python"
#     handler:parser:handler

def handler(context, event):
	pass
`
	tmpFile, err := ioutil.TempFile("", "nuclio-parser-test")
	suite.Require().NoError(err)
	suite.Require().NoError(tmpFile.Close())

	defer os.Remove(tmpFile.Name()) // nolint: errcheck

	err = ioutil.WriteFile(tmpFile.Name(), []byte(content), 0600)
	suite.Require().NoError(err)

	blocks, err := suite.parser.Parse(tmpFile.Name())
	suite.Require().NoError(err)

	// get function YAML
	suite.Require().NotEmpty(blocks["configure"])
	suite.Require().Error(blocks["configure"].Error)
	suite.Require().Equal(blocks["configure"].RawContents, `
function.yaml:
  spec:
    runtime: "python"
    handler:parser:handler
`)
}

func TestInlineParserTestSuite(t *testing.T) {
	suite.Run(t, new(InlineParserTestSuite))
}
