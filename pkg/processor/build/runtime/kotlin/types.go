/*
Copyright 2018 The Nuclio Authors.

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

package kotlin

import (
	"fmt"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/ghodss/yaml"
	"github.com/mitchellh/mapstructure"
)

type dependency struct {
	Group   string `json:"group,omitempty"`
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

func newDependency(raw string) (*dependency, error) {
	newDependency := dependency{}

	// enclose in curly brackets
	raw = fmt.Sprintf("{%s}", raw)

	if err := yaml.Unmarshal([]byte(raw), &newDependency); err != nil {
		return nil, errors.Wrapf(err, "Failed to parse dependency: %s", raw)
	}

	return &newDependency, nil
}

type buildAttributes struct {
	Repositories []string
}

func newBuildAttributes(encodedBuildAttributes map[string]interface{}) (*buildAttributes, error) {
	newBuildAttributes := buildAttributes{}

	// parse attributes
	if err := mapstructure.Decode(encodedBuildAttributes, &newBuildAttributes); err != nil {
		return nil, errors.Wrap(err, "Failed to decode build attributes")
	}

	if len(newBuildAttributes.Repositories) == 0 {
		newBuildAttributes.Repositories = []string{
			"mavenCentral()",
		}
	}

	return &newBuildAttributes, nil
}
