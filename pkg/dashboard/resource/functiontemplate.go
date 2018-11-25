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
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/dashboard/functiontemplates"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/fatih/structs"
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

			attributes[matchingFunctionTemplate.FunctionConfig.Meta.Name] = restful.Attributes{
				"metadata": matchingFunctionTemplate.FunctionConfig.Meta,
				"template": matchingFunctionTemplate.FunctionConfigTemplate,
				"values":   matchingFunctionTemplate.FunctionConfigValues,
			}
		} else {
			renderedValues := make(map[string]interface{})
			renderedValues["metadata"] = matchingFunctionTemplate.FunctionConfig.Meta
			renderedValues["spec"] = matchingFunctionTemplate.FunctionConfig.Spec

			// add to attributes
			attributes[matchingFunctionTemplate.FunctionConfig.Meta.Name] = restful.Attributes{
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

	renderGivenValues := functiontemplates.RenderConfig{}
	err = json.Unmarshal(body, &renderGivenValues)
	if err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to parse JSON body"))
	}

	renderer := functiontemplates.NewFunctionTemplateRenderer(ftr.Logger)
	functionConfig, err := renderer.Render(&renderGivenValues)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to render request body")
	}

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
