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

package functiontemplates

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type testSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *testSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
}

func (suite *testSuite) TestFunctionTemplateRender() {
	minReplicas := 1
	maxReplicas := 2
	expectedFunctionConfig := &functionconfig.Config{
		Spec: functionconfig.Spec{
			Handler:     "myhandler",
			MinReplicas: &minReplicas,
			MaxReplicas: &maxReplicas,
			Runtime:     "python:3.6",
		},
	}

	functionTemplate := `apiVersion: \"nuclio.io/v1beta1\"\nkind: \"Function\"\nspec:\n  runtime: \"python:3.6\"\n` +
		`  handler: {{ .handler }}\n  minReplicas: {{ .minReplicas }}\n  maxReplicas: {{ .maxReplicas }}`

	functionTemplateConfig := []byte(fmt.Sprintf(`{
"template": "%s",
"source_code": "def handler(context, event):\n    return ''",
"values": {
	"handler": "myhandler",
 	"maxReplicas": 2,
	"minReplicas": 1
}
}`, functionTemplate))

	renderGivenValues := RenderConfig{}
	err := json.Unmarshal(functionTemplateConfig, &renderGivenValues)
	suite.Require().NoError(err)

	renderer := NewFunctionTemplateRenderer(suite.logger)
	result, err := renderer.Render(&renderGivenValues)
	suite.Require().NoError(err)

	suite.Require().Equal(expectedFunctionConfig, result)
}

func TestTemplateRender(t *testing.T) {
	suite.Run(t, new(testSuite))
}
