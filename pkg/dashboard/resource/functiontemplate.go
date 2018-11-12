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
	"io/ioutil"
	"net/http"

	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/dashboard/functiontemplates"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/fatih/structs"
	"github.com/icza/dyno"
	"github.com/nuclio/nuclio-sdk-go"
)

type functionTemplateResource struct {
	*resource
	functionTemplateRepository *functiontemplates.Repository
}

func (ftr *functionTemplateResource) OnAfterInitialize() error {
	ftr.functionTemplateRepository = ftr.resource.GetServer().(*dashboard.Server).Repository
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
			values := make(map[string]interface{})

			for valueName, valueInterface := range values {
				values[valueName] = dyno.ConvertMapI2MapS(valueInterface)
			}

			attributes[matchingFunctionTemplate.Name] = restful.Attributes{
				"template": matchingFunctionTemplate.FunctionConfigTemplate,
				"values":   values,
			}
		} else {
			renderedValues := make(map[string]interface{}, 2)
			renderedValues["meta"] = matchingFunctionTemplate.FunctionConfig.Meta
			renderedValues["spec"] = matchingFunctionTemplate.FunctionConfig.Spec

			// add to attributes
			attributes[matchingFunctionTemplate.Name] = restful.Attributes{
				"rendered": renderedValues,
			}
		}
	}

	return attributes, nil
}

// returns a list of custom routes for the resource
func (ftr *functionTemplateResource) GetCustomRoutes() ([]restful.CustomRoute, error) {
	return []restful.CustomRoute{
		{
			Pattern:   "/render",
			Method:    http.MethodPost,
			RouteFunc: ftr.render,
		},
	}, nil
}

func (ftr *functionTemplateResource) resourceToAttributes(resource interface{}) restful.Attributes {

	s := structs.New(resource)

	// use "json" tag to specify how to serialize the keys
	s.TagName = "json"

	return s.Map()
}

func (ftr *functionTemplateResource) render(request *http.Request) (*restful.CustomRouteFuncResponse, error) {
	statusCode := http.StatusOK

	// read body
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return nil, nuclio.WrapErrInternalServerError(errors.Wrap(err, "Failed to read body"))
	}

	functionConfig, err := functiontemplates.Render(body)

	// return the stuff
	return &restful.CustomRouteFuncResponse{
		ResourceType: "functionTemplate",
		Resources: map[string]restful.Attributes{
			"functionConfig": ftr.resourceToAttributes(functionConfig),
		},
		Single:     true,
		StatusCode: statusCode,
	}, err
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
