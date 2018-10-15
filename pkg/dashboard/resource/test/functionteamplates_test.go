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

package test

import (
	"os"
	"testing"

	"github.com/nuclio/nuclio/pkg/dashboard/functiontemplates"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/ghodss/yaml"
)

func TestProcessTemplates(t *testing.T) {
	githuAPItoken := os.Getenv("PROVAZIO_GITHUB_API_TOKEN")
	supportedSuffixes := []string{".go", ".py"}

	templateFetcher, err := functiontemplates.NewGithubFunctionTemplateFetcher("nuclio-templates", "ilaykav", "master", githuAPItoken, supportedSuffixes)
	if err != nil {
		t.Error(err)
	}

	templates, err := templateFetcher.Fetch()
	if err != nil {
		t.Error(err)
	}
	attributes := map[string]restful.Attributes{}

	for _, matchingFunctionTemplate := range templates {

		// if not rendered, add template in "values" mode, else just add as functionConfig with Meta and Spec
		if matchingFunctionTemplate.FunctionConfigTemplate != "" {
			var values map[string]interface{}
			err := yaml.Unmarshal([]byte(matchingFunctionTemplate.FunctionConfigValues), &values)
			if err != nil {
				t.Error("Failed to unmarshall function template's values file", err)
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

	t.Log("Fetcher ended", "attributes", attributes)
}
