package functiontemplates

import (
	"encoding/base64"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/logger"
)

type Repository struct {
	logger            logger.Logger
	functionTemplates []*FunctionTemplate
}

func NewRepository(parentLogger logger.Logger, fetchers []FunctionTemplateFetcher) (*Repository, error) {
	var templates []*FunctionTemplate

	for _, fetcher := range fetchers {
		currentFetcherTemplates, err := fetcher.Fetch()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to fetch one of given templateFetchers")
		}

		for _, template := range currentFetcherTemplates {
			templates = append(templates, &template)
		}
	}

	newRepository := &Repository{
		logger:            parentLogger.GetChild("repository"),
		functionTemplates: templates,
	}

	// populate encoded field of templates so that when we are queried we have this ready
	if err := newRepository.enrichFunctionTemplates(newRepository.functionTemplates); err != nil {
		return nil, errors.Wrap(err, "Failed to populated serialized templates")
	}

	return newRepository, nil
}

func (r *Repository) GetFunctionTemplates(filter *Filter) []*FunctionTemplate {
	var passingFunctionTemplates []*FunctionTemplate

	for _, functionTemplate := range r.functionTemplates {
		if filter == nil || filter.functionTemplatePasses(functionTemplate) {
			passingFunctionTemplates = append(passingFunctionTemplates, functionTemplate)
		}
	}

	return passingFunctionTemplates
}

func (r *Repository) enrichFunctionTemplates(functionTemplates []*FunctionTemplate) error {
	for _, functionTemplate := range functionTemplates {

		// set name
		functionTemplate.FunctionConfig.Meta.Name = functionTemplate.Name

		// encode source code into configuration
		functionTemplate.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString(
			[]byte(functionTemplate.SourceCode))

		// what to do with this?
		//functionTemplate.serializedTemplate, err = yaml.Marshal(functionTemplate.Configuration)
		//if err != nil {
		//	return errors.Wrap(err, "Failed to serialize configuration")
		//}
	}

	return nil
}
