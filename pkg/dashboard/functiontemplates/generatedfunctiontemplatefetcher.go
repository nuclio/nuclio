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

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/ghodss/yaml"
	"github.com/nuclio/logger"
)

type GeneratedFunctionTemplateFetcher struct {
	logger            logger.Logger
	functionTemplates []*FunctionTemplate
}

func NewGeneratedFunctionTemplateFetcher(parentLogger logger.Logger) (*GeneratedFunctionTemplateFetcher, error) {
	generatedFunctionTemplates := GeneratedFunctionTemplates
	generatedFunctionTemplateFetcher := &GeneratedFunctionTemplateFetcher{
		logger: parentLogger.GetChild("generatedFunctionTemplateFetcher"),
	}

	// populate encoded field of templates so that when we are queried we have this ready
	if err := enrichFunctionTemplates(generatedFunctionTemplates); err != nil {
		return nil, errors.Wrap(err, "Failed to populated serialized templates")
	}

	err := generatedFunctionTemplateFetcher.SetGeneratedFunctionTemplates(generatedFunctionTemplates)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to set functionTemplates out of generatedFunctionTemplates")
	}

	return generatedFunctionTemplateFetcher, nil
}

func (gftf *GeneratedFunctionTemplateFetcher) SetGeneratedFunctionTemplates(generatedFunctionTemplates []*generatedFunctionTemplate) error {

	// populate encoded field of templates so that when we are queried we have this ready
	if err := enrichFunctionTemplates(generatedFunctionTemplates); err != nil {
		return errors.Wrap(err, "Failed to populated serialized templates")
	}

	functionTemplatesFromGeneratedFunctionTemplates, err := gftf.generatedFunctionTemplatesToFunctionTemplates(generatedFunctionTemplates)
	if err != nil {
		return errors.Wrap(err, "Failed to generate regular functionTemplates out og generatedFunctionTemplates")
	}

	gftf.functionTemplates = functionTemplatesFromGeneratedFunctionTemplates
	return nil
}

func (gftf *GeneratedFunctionTemplateFetcher) Fetch() ([]*FunctionTemplate, error) {
	var returnFunctionTemplates []*FunctionTemplate
	for functionTemplateIndex := 0; functionTemplateIndex < len(gftf.functionTemplates); functionTemplateIndex++ {
		returnFunctionTemplates = append(returnFunctionTemplates, gftf.functionTemplates[functionTemplateIndex])
	}
	return returnFunctionTemplates, nil
}

func (gftf *GeneratedFunctionTemplateFetcher) generatedFunctionTemplatesToFunctionTemplates(generatedFunctionTemplates []*generatedFunctionTemplate) ([]*FunctionTemplate, error) {
	functionTemplates := make([]*FunctionTemplate, len(generatedFunctionTemplates))
	for generatedFunctionTemplateIndex := 0; generatedFunctionTemplateIndex < len(generatedFunctionTemplates); generatedFunctionTemplateIndex++ {
		functionTemplates[generatedFunctionTemplateIndex] = &FunctionTemplate{
			SourceCode:             generatedFunctionTemplates[generatedFunctionTemplateIndex].SourceCode,
			Name:                   generatedFunctionTemplates[generatedFunctionTemplateIndex].Name,
			FunctionConfig:         &generatedFunctionTemplates[generatedFunctionTemplateIndex].Configuration,
			DisplayName:            generatedFunctionTemplates[generatedFunctionTemplateIndex].DisplayName,
			serializedTemplate:     generatedFunctionTemplates[generatedFunctionTemplateIndex].serializedTemplate,
			FunctionConfigValues:   map[string]interface{}{},
			FunctionConfigTemplate: "",
		}
	}

	return functionTemplates, nil
}

func enrichFunctionTemplates(functionTemplates []*generatedFunctionTemplate) error {
	var err error

	for _, functionTemplate := range functionTemplates {

		// set name
		functionTemplate.Configuration.Meta.Name = functionTemplate.Name

		// encode source code into configuration
		functionTemplate.Configuration.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString(
			[]byte(functionTemplate.SourceCode))

		functionTemplate.serializedTemplate, err = yaml.Marshal(functionTemplate.Configuration)
		if err != nil {
			return errors.Wrap(err, "Failed to serialize configuration")
		}
	}

	return nil
}
