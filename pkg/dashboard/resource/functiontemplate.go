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
	"os"

	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/dashboard/functiontemplates"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/fatih/structs"
	"gopkg.in/yaml.v2"
	"github.com/nuclio/nuclio-sdk-go"
	"encoding/json"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"strings"
)

type functionTemplateResource struct {
	*resource
	functionTemplateRepository *functiontemplates.Repository
}

type RenderConfig struct {
	Template string `json:"template,omitempty"`
	Values map[string]interface{} `json:"values,omitempty"`
}

func (ftr *functionTemplateResource) OnAfterInitialize() error {
	githuAPItoken := os.Getenv("NUCLIO_GITHUB_API_TOKEN")
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

// returns a list of custom routes for the resource
func (ftr *functionTemplateResource) GetCustomRoutes() ([]restful.CustomRoute, error) {

	// since delete and update by default assume /resource/{id} and we want to get the id/namespace from the body
	// we need to register custom routes
	return []restful.CustomRoute{
		{
			Pattern:   "/render",
			Method:    http.MethodPost,
			RouteFunc: ftr.Render,
		},
	}, nil
}

func getTemplateFileWithValues (templateFile string, values map[string]interface{}) string {
	for valueName, valueInterface := range values {
		templateFile = strings.Replace(templateFile, "{{ ." + valueName + " }}", valueInterface.(string), -1)
	}
	return templateFile
}

func getFunctionConfigFromTemplate (templateFile string) (*functionconfig.Config, error) {
	funcitonConfig := functionconfig.Config{}

	err := json.Unmarshal([]byte(templateFile), &funcitonConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshall function config from template file")
	}

	return &funcitonConfig, nil
}

func (ftr *functionTemplateResource) resourceToAttributes(resource interface{}) restful.Attributes {

	s := structs.New(resource)

	// use "json" tag to specify how to serialize the keys
	s.TagName = "json"

	return s.Map()
}

func (ftr *functionTemplateResource) Render(request *http.Request) (*restful.CustomRouteFuncResponse, error) {

	statusCode := http.StatusNoContent

	// read body
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return nil, nuclio.WrapErrInternalServerError(errors.Wrap(err, "Failed to read body"))
	}

	renderGivenValues := RenderConfig{}
	err = json.Unmarshal(body, &renderGivenValues)
	if err != nil {
		return nil, nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to parse JSON body"))
	}

	// from values to template
	renderGivenValues.Template = getTemplateFileWithValues(renderGivenValues.Template, renderGivenValues.Values)

	// from template to functionConfig
	functionConfig, err := getFunctionConfigFromTemplate(renderGivenValues.Template)

	if err != nil {
		return nil , nuclio.WrapErrBadRequest(errors.Wrap(err, "Failed to get functionConfig from template"))
	}

	// return the stuff
	return &restful.CustomRouteFuncResponse{
		ResourceType: "functionTemplate",
		Resources: map[string]restful.Attributes{
			"functionConfig": ftr.resourceToAttributes(functionConfig),
		},
		Single:       true,
		StatusCode:   statusCode,
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
