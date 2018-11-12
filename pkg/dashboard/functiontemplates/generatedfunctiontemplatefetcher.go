package functiontemplates

import (
	"encoding/base64"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/ghodss/yaml"
)

type GeneratedFunctionTemplateFetcher struct {
	functionTemplates []*FunctionTemplate
	FunctionTemplateFetcher
}

func NewGeneratedFunctionTemplateFetcher() (*GeneratedFunctionTemplateFetcher, error) {
	generatedFunctionTemplates := GeneratedFunctionTemplates

	// populate encoded field of templates so that when we are queried we have this ready
	if err := enrichFunctionTemplates(generatedFunctionTemplates); err != nil {
		return nil, errors.Wrap(err, "Failed to populated serialized templates")
	}

	functionTemplatesFromGeneratedFunctionTemplates, err := generatedFunctionTemplatesToFunctionTemplates(generatedFunctionTemplates)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to generate regular functionTemplates out og generatedFunctionTemplates")
	}

	return &GeneratedFunctionTemplateFetcher{
		functionTemplates: functionTemplatesFromGeneratedFunctionTemplates,
	}, nil
}

func NewGeneratedFunctionTemplateFetcherFromTemplates(generatedFunctionTemplates []*generatedFunctionTemplate) (*GeneratedFunctionTemplateFetcher, error) {

	// populate encoded field of templates so that when we are queried we have this ready
	if err := enrichFunctionTemplates(generatedFunctionTemplates); err != nil {
		return nil, errors.Wrap(err, "Failed to populated serialized templates")
	}

	functionTemplatesFromGeneratedFunctionTemplates, err := generatedFunctionTemplatesToFunctionTemplates(generatedFunctionTemplates)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to generate regular functionTemplates out og generatedFunctionTemplates")
	}

	return &GeneratedFunctionTemplateFetcher{
		functionTemplates: functionTemplatesFromGeneratedFunctionTemplates,
	}, nil
}

func (gftf *GeneratedFunctionTemplateFetcher) Fetch() ([]FunctionTemplate, error) {
	returnFunctionTemplates := make([]FunctionTemplate, len(gftf.functionTemplates))
	for functionTemplateIndex := 0; functionTemplateIndex < len(gftf.functionTemplates); functionTemplateIndex++ {
		returnFunctionTemplates[functionTemplateIndex] = *gftf.functionTemplates[functionTemplateIndex]
	}
	return returnFunctionTemplates, nil
}

func generatedFunctionTemplatesToFunctionTemplates(generatedFunctionTemplates []*generatedFunctionTemplate) ([]*FunctionTemplate, error) {
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
