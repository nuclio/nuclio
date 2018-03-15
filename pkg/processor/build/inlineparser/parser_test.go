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
	"strings"
	"testing"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v2"
	"github.com/nuclio/nuclio/pkg/processor/build/inlineparser"
	"fmt"
)

type InlineParserTestSuite struct {
	suite.Suite
	logger logger.Logger
	parser *inlineparser.Parser
}

func (suite *InlineParserTestSuite) SetupTest() {
	var err error

	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.parser, err = inlineparser.NewParser(suite.logger)
	if err != nil {
		panic("Failed to create command runner")
	}
}


func (suite *InlineParserTestSuite) TestValidBlockSingleChar() {
	contentReader := strings.NewReader(`
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# @nuclio.createFiles
#
# processor.yaml:
#   function:
#     kind: "python"
#     python_version: "3"
#     handler: parser:handler
#
# build.yaml:
#   commands:
#     - pip install simplejson
#

import simplejson

def handler(context, event):
    """Return a field from within a json"""

    context.logger.info('Hello from Python')
    body = simplejson.loads(event.body.decode('utf-8'))
    return body['return_this']
`)


	expectedValues:= "kind:python python_version:3 handler:parser:handler"

	blocks, err := suite.parser.Parse(contentReader, "#")
	suite.Require().NoError(err)

	processorYaml := blocks["createFiles"]["processor.yaml"]
	yaml.Marshal(processorYaml)

	actualMap := fmt.Sprintf("%v", blocks["createFiles"]["processor.yaml"])

	suite.Assert().EqualValues(strings.ContainsAny(actualMap,expectedValues), true)


}


func (suite *InlineParserTestSuite) TestEmptyBlockSingleChar() {
	contentReader := strings.NewReader(`
# @nuclio.createFiles
#
# processor.yaml:

`)


	blocks, err := suite.parser.Parse(contentReader, "#")
	suite.Require().NoError(err)

	processorYaml := blocks["createFiles"]["processor.yaml"]
	yaml.Marshal(processorYaml)

	suite.Assert().EqualValues(blocks["createFiles"]["processor.yaml"], nil)
}


func (suite *InlineParserTestSuite) TestAbsentOfNuclioAnnotationChars() {
	contentReader := strings.NewReader(`
`)


	blocks, err := suite.parser.Parse(contentReader, "#")
	suite.Require().NoError(err)

	processorYaml := blocks["createFiles"]["processor.yaml"]
	yaml.Marshal(processorYaml)

	suite.Assert().EqualValues(blocks["createFiles"]["processor.yaml"], nil)
}



func TestInlineParserTestSuite(t *testing.T) {
	suite.Run(t, new(InlineParserTestSuite))
}
