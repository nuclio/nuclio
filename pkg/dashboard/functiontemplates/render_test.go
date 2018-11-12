package functiontemplates

import (
	"fmt"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/stretchr/testify/suite"
)

type testSuite struct {
	suite.Suite
}

func (suite *testSuite) TestFunctionTemplateRender() {
	expectedFunctionConfig := &functionconfig.Config{
		Spec: functionconfig.Spec{
			Handler:     "myhandler",
			MinReplicas: 1,
			MaxReplicas: 2,
			Runtime:     "python:3.6",
		},
	}

	functionTemplate := `apiVersion: \"nuclio.io/v1beta1\"\nkind: \"Function\"\nspec:\n  runtime: \"python:3.6\"\n` +
		`handler: {{ .handler }}\n  minReplicas: {{ .minReplicas }}\n  maxReplicas: {{ .maxReplicas }}`

	functionTemplateConfig := fmt.Sprintf(`{
"template": "%s",
"source_code": "def handler(context, event):\n    return ''",
"values": {
	"handler": "myhandler",
 	"maxReplicas": 2,
	"minReplicas": 1
}
}`, functionTemplate)

	result, err := Render([]byte(functionTemplateConfig))
	suite.Require().NoError(err)

	suite.Require().Equal(expectedFunctionConfig, result)
}

func TestTemplateRender(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(testSuite))
}
