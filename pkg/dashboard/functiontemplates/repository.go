package functiontemplates

import (
	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/logger"
)

type Repository struct {
	logger            logger.Logger
	functionTemplates []*functionTemplate
}

func NewRepository(parentLogger logger.Logger, fetchers []FunctionTemplateFetcher) (*Repository, error) {
	var templates []*functionTemplate

	for _, fetcher := range fetchers {
		currentFetcherTemplates, err := fetcher.Fetch()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to fetch one of given templateFetchers")
		}

		for templateIndex := 0; templateIndex < len(currentFetcherTemplates); templateIndex++ {
			templates = append(templates, &currentFetcherTemplates[templateIndex])
		}
	}

	newRepository := &Repository{
		logger:            parentLogger.GetChild("repository"),
		functionTemplates: templates,
	}

	return newRepository, nil
}

func (r *Repository) GetFunctionTemplates(filter *Filter) []*functionTemplate {
	var passingFunctionTemplates []*functionTemplate

	for _, functionTemplate := range r.functionTemplates {
		if filter == nil || filter.functionTemplatePasses(functionTemplate) {
			passingFunctionTemplates = append(passingFunctionTemplates, functionTemplate)
		}
	}

	return passingFunctionTemplates
}