package functiontemplates

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"encoding/base64"
	"github.com/ghodss/yaml"
)

type generatedFunctionTemplateFetcher struct {
	functionTemplates []*functionTemplate
	FunctionTemplateFetcher
}

func NewGeneratedFunctionTemplateFetcher() (*generatedFunctionTemplateFetcher, error) {
	generatedFunctionTemplates := FunctionTemplates

	// populate encoded field of templates so that when we are queried we have this ready
	if err := enrichFunctionTemplates(generatedFunctionTemplates); err != nil {
		return nil, errors.Wrap(err, "Failed to populated serialized templates")
	}

	functionTemplatesFromGeneratedFunctionTemplates, err := generatedFunctionTemplatesdToFunctionTemplates(generatedFunctionTemplates)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to generate regular functionTemplates out og generatedFunctionTemplates")
	}

	return &generatedFunctionTemplateFetcher{
		functionTemplates: functionTemplatesFromGeneratedFunctionTemplates,
	}, nil
}

func (gftf *generatedFunctionTemplateFetcher) Fetch() ([]functionTemplate, error) {
	returnFunctionTemplates := make([]functionTemplate, len(gftf.functionTemplates))
	for functionTemplateIndex := 0; functionTemplateIndex < len(gftf.functionTemplates); functionTemplateIndex++ {
		returnFunctionTemplates[functionTemplateIndex] = *gftf.functionTemplates[functionTemplateIndex]
	}
	return returnFunctionTemplates, nil
}

func generatedFunctionTemplatesdToFunctionTemplates (generatedFunctionTemplates []*generatedFunctionTemplate) ([]*functionTemplate, error) {
	functionTemplates := make([]*functionTemplate, len(generatedFunctionTemplates))
	for generatedFunctionTemplateIndex := 0; generatedFunctionTemplateIndex < len(generatedFunctionTemplates); generatedFunctionTemplateIndex++ {
		functionTemplates[generatedFunctionTemplateIndex] = &functionTemplate{
			SourceCode: generatedFunctionTemplates[generatedFunctionTemplateIndex].SourceCode,
			Name: generatedFunctionTemplates[generatedFunctionTemplateIndex].Name,
			FunctionConfig: &generatedFunctionTemplates[generatedFunctionTemplateIndex].Configuration,
			DisplayName: generatedFunctionTemplates[generatedFunctionTemplateIndex].DisplayName,
			serializedTemplate: generatedFunctionTemplates[generatedFunctionTemplateIndex].serializedTemplate,
			FunctionConfigValues: "",
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
			return errors.Wrap(err, "Fauled to serialize configuration")
		}
	}

	return nil
}
