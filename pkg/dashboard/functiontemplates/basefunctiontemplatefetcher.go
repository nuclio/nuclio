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
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/ghodss/yaml"
	"github.com/icza/dyno"
	"github.com/nuclio/errors"
	"github.com/rs/xid"
)

type BaseFunctionTemplateFetcher struct {
}

func (bftf *BaseFunctionTemplateFetcher) createFunctionTemplate(ftfc FunctionTemplateFileContents,
	functionName string) (*FunctionTemplate, error) {

	functionTemplate := FunctionTemplate{}

	functionTemplate.Name = functionName
	functionTemplate.SourceCode = ftfc.Code

	if ftfc.Template != "" && ftfc.Values != "" {
		if err := bftf.enrichFunctionConfig(&functionTemplate, ftfc.Template, ftfc.Values); err != nil {
			return nil, errors.Wrap(err, "Failed to enrich function config")
		}
	} else {

		// The given file contents are not of a valid function template
		return nil, nil
	}

	return &functionTemplate, nil
}

func (bftf *BaseFunctionTemplateFetcher) replaceSourceCodeInTemplate(functionTemplate *FunctionTemplate) {

	// hack: if template writer passed a function source code, reflect it in template by replacing `functionSourceCode: {{ .SourceCode }}`
	replacement := fmt.Sprintf("functionSourceCode: %s",
		base64.StdEncoding.EncodeToString([]byte(functionTemplate.SourceCode)))
	pattern := "functionSourceCode: {{ .SourceCode }}"
	functionTemplate.FunctionConfigTemplate = strings.Replace(functionTemplate.FunctionConfigTemplate,
		pattern,
		replacement,
		1)
}

func (bftf *BaseFunctionTemplateFetcher) enrichFunctionTemplate(functionTemplate *FunctionTemplate) {

	// set the source code we got earlier
	functionTemplate.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString(
		[]byte(functionTemplate.SourceCode))

	// set something unique, the UI will ignore everything after `:`, this is par to pre-generated templates
	functionTemplate.FunctionConfig.Meta = functionconfig.Meta{
		Name: functionTemplate.Name + ":" + xid.New().String(),
	}
}

func (bftf *BaseFunctionTemplateFetcher) enrichFunctionConfig(functionTemplate *FunctionTemplate, templateFile string, valuesFile string) error {
	functionTemplate.FunctionConfigTemplate = templateFile

	var values map[string]interface{}
	if err := yaml.Unmarshal([]byte(valuesFile), &values); err != nil {
		return errors.Wrap(err, "Failed to unmarshall function template's values file")
	}

	for valueName, valueInterface := range values {
		values[valueName] = dyno.ConvertMapI2MapS(valueInterface)
	}

	functionTemplate.FunctionConfigValues = values
	functionTemplate.FunctionConfig = &functionconfig.Config{}

	if functionTemplate.SourceCode != "" {
		bftf.replaceSourceCodeInTemplate(functionTemplate)
	}
	bftf.enrichFunctionTemplate(functionTemplate)
	return nil
}
