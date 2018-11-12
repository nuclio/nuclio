package functiontemplates

import (
	"bytes"
	"text/template"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/ghodss/yaml"
	"github.com/nuclio/nuclio-sdk-go"
	"k8s.io/apimachinery/pkg/util/json"
)

type RenderConfig struct {
	Template string                 `json:"template,omitempty"`
	Values   map[string]interface{} `json:"values,omitempty"`
}

func Render(contents []byte) (*functionconfig.Config, error) {
	renderGivenValues := RenderConfig{}
	err := json.Unmarshal(contents, &renderGivenValues)
	if err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to parse JSON body"))
	}

	// from template to functionConfig
	functionConfig, err := getFunctionConfigFromTemplateAndValues(renderGivenValues.Template, renderGivenValues.Values)

	if err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to get functionConfig from template"))
	}

	return functionConfig, nil
}

func getFunctionConfigFromTemplateAndValues(templateFile string, values map[string]interface{}) (*functionconfig.Config, error) {
	functionConfig := functionconfig.Config{}

	// create new template
	functionConfigTemplate, err := template.New("functionConfig template").Parse(templateFile)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse templateFile")
	}

	functionConfigBuffer := bytes.Buffer{}

	// use template and values to make combined config string
	err = functionConfigTemplate.Execute(&functionConfigBuffer, values)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse templateFile")
	}

	// unmarshal this string into functionConfig
	err = yaml.Unmarshal([]byte(functionConfigBuffer.String()), &functionConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal functionConfigBuffer into functionConfig")
	}

	return &functionConfig, nil
}
