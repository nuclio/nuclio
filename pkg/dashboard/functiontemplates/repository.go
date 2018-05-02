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

type Repository struct {
	logger            logger.Logger
	functionTemplates []*FunctionTemplate
}

func NewRepository(parentLogger logger.Logger, functionTemplates []*FunctionTemplate) (*Repository, error) {
	newRepository := &Repository{
		logger:            parentLogger.GetChild("repository"),
		functionTemplates: functionTemplates,
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
