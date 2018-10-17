package functiontemplates

import (
	"github.com/nuclio/nuclio/pkg/errors"
)

type generatedFunctionTemplateFetcher struct {

	FunctionTemplateFetcher
}

func NewGeneratedFunctionTemplateFetcher() (*githubFunctionTemplateFetcher, error) {
	return &githubFunctionTemplateFetcher{
	}, nil
}

func (gftf *generatedFunctionTemplateFetcher) Fetch() ([]functionTemplate, error) {
	var passingFunctionTemplates []*functionTemplate

	for _, functionTemplate := range r.functionTemplates {
		if filter == nil || filter.functionTemplatePasses(functionTemplate) {
			passingFunctionTemplates = append(passingFunctionTemplates, functionTemplate)
		}
}

	return passingFunctionTemplates
}
