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

func (r *Repository) GetFunctionTemplates(filter *Filter) []*FunctionTemplate {
	var passingFunctionTemplates []*FunctionTemplate

	for _, functionTemplate := range r.functionTemplates {
		if filter == nil || filter.functionTemplatePasses(functionTemplate) {
			passingFunctionTemplates = append(passingFunctionTemplates, functionTemplate)
		}
	}

	return passingFunctionTemplates
}
