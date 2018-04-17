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

	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/dashboard/functiontemplates"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/restful"
)

type functionTemplateResource struct {
	*resource
	functionTemplateRepository *functiontemplates.Repository
}

func (ftr *functionTemplateResource) OnAfterInitialize() error {
	var err error

	// repository will hold a repository of function templates
	ftr.functionTemplateRepository, err = functiontemplates.NewRepository(ftr.Logger, functiontemplates.FunctionTemplates)
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

		// add to attributes
		attributes[matchingFunctionTemplate.Name] = restful.Attributes{
			"metadata": matchingFunctionTemplate.Configuration.Meta,
			"spec":     matchingFunctionTemplate.Configuration.Spec,
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
