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
	"bytes"
	"text/template"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/ghodss/yaml"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type FunctionTemplateRenderer struct {
	logger logger.Logger
}

type RenderConfig struct {
	Template string                 `json:"template,omitempty"`
	Values   map[string]interface{} `json:"values,omitempty"`
}

func NewFunctionTemplateRenderer(parentLogger logger.Logger) *FunctionTemplateRenderer {
	return &FunctionTemplateRenderer{
		logger: parentLogger.GetChild("renderer"),
	}
}

func (r *FunctionTemplateRenderer) Render(renderGivenValues *RenderConfig) (*functionconfig.Config, error) {

	// from template to functionConfig
	functionConfig, err := r.getFunctionConfigFromTemplateAndValues(renderGivenValues.Template, renderGivenValues.Values)

	if err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to get functionConfig from template"))
	}

	return functionConfig, nil
}

func (r *FunctionTemplateRenderer) getFunctionConfigFromTemplateAndValues(templateFile string,
	values map[string]interface{}) (*functionconfig.Config, error) {
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
	err = yaml.Unmarshal(functionConfigBuffer.Bytes(), &functionConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal functionConfigBuffer into functionConfig")
	}

	return &functionConfig, nil
}
