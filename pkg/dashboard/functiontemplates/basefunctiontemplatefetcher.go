package functiontemplates

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/icza/dyno"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/rs/xid"
)

type BaseFunctionTemplateFetcher struct {
	logger     logger.Logger
}

func (bftf *BaseFunctionTemplateFetcher) createFunctionTemplate(ftfc FunctionTemplateFileContents,
	functionName string) (*FunctionTemplate, error) {

	functionTemplate := FunctionTemplate{}

	functionTemplate.Name = functionName
	functionTemplate.SourceCode = ftfc.Code

	// Check and handle according to the given type of function template structure (more details in FunctionTemplateFileContents type struct)
	if ftfc.TemplateAndValues != "" {
		if err := bftf.enrichFunctionConfigFirstType(&functionTemplate, ftfc.TemplateAndValues); err != nil {
			return nil, errors.Wrap(err, "Failed to enrich function config (first type)")
		}
	} else if ftfc.Template != "" && ftfc.Values != "" {
		if err := bftf.enrichFunctionConfigSecondType(&functionTemplate, ftfc.Template, ftfc.Values); err != nil {
			return nil, errors.Wrap(err, "Failed to enrich function config (second type)")
		}
	} else {
		return nil, errors.New("Unknown function file contents structure")
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

func (bftf *BaseFunctionTemplateFetcher) enrichFunctionConfigFirstType(functionTemplate *FunctionTemplate, functionFile string) error {
	err := yaml.Unmarshal([]byte(functionFile), functionTemplate.FunctionConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to unmarshall yaml file function.yaml")
	}

	bftf.enrichFunctionTemplate(functionTemplate)
	return nil
}

func (bftf *BaseFunctionTemplateFetcher) enrichFunctionConfigSecondType(functionTemplate *FunctionTemplate, templateFile string, valuesFile string) error {
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

	bftf.replaceSourceCodeInTemplate(functionTemplate)
	bftf.enrichFunctionTemplate(functionTemplate)
	return nil
}
