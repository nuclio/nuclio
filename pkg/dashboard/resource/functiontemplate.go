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

package resource

import (
	"net/http"
	"os"

	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/dashboard/functiontemplates"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/restful"

	"gopkg.in/yaml.v2"
)

type functionTemplateResource struct {
	*resource
	functionTemplateRepository *functiontemplates.Repository
}

func (ftr *functionTemplateResource) OnAfterInitialize() error {
	githuAPItoken := os.Getenv("PROVAZIO_GITHUB_API_TOKEN")
	supportedSuffixes := []string{".go", ".py"}

	repoFetcher, err := functiontemplates.NewGithubFunctionTemplateFetcher("nuclio-templates", "ilaykav", "master", githuAPItoken, supportedSuffixes)
	if err != nil {
		return errors.Wrap(err, "Failed to create github fetcher")
	}

	// repository will hold a repository of function templates
	ftr.functionTemplateRepository, err = functiontemplates.NewRepository(ftr.Logger, []functiontemplates.FunctionTemplateFetcher{repoFetcher})
	if err != nil {
		return errors.Wrap(err, "Failed to create repository")
	}

	return nil
}

// GetAll returns all functionTemplates
func (ftr *functionTemplateResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {
	attributes := map[string]restful.Attributes{}

	// create filter
	filter := functiontemplates.Filter{
		Contains: request.Header.Get("x-nuclio-filter-contains"),
	}

	// get all templates that pass a certain filter
	matchingFunctionTemplates := ftr.functionTemplateRepository.GetFunctionTemplates(&filter)

	for _, matchingFunctionTemplate := range matchingFunctionTemplates {

		// if not rendered, add template in "values" mode, else just add as functionConfig with Meta and Spec
		if matchingFunctionTemplate.FunctionConfigTemplate != "" {
			var values map[string]interface{}
			err := yaml.Unmarshal([]byte(matchingFunctionTemplate.FunctionConfigValues), &values)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to unmarshall function template's values file")
			}

			attributes[matchingFunctionTemplate.Name] = restful.Attributes{
				"template": matchingFunctionTemplate.FunctionConfigTemplate,
				"values":   values,
			}
		} else {

			// add to attributes
			attributes[matchingFunctionTemplate.Name] = restful.Attributes{
				"metadata": matchingFunctionTemplate.FunctionConfig.Meta,
				"spec":     matchingFunctionTemplate.FunctionConfig.Spec,
			}
		}
	}

	return attributes, nil
}

// register the resource
var functionTemplateResourceInstance = &functionTemplateResource{
	resource: newResource("api/function_templates", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
	}),
}

func init() {
	functionTemplateResourceInstance.Resource = functionTemplateResourceInstance
	functionTemplateResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
