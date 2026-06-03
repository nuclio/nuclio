/*
Copyright 2023 The Nuclio Authors.

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

package java

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
	"sigs.k8s.io/yaml"
)

// repositoryPattern matches a single no-argument Gradle repository declaration of the form
// "name()" (e.g. "mavenCentral()"). Repository values are rendered verbatim into the
// generated build.gradle, so the value must not be able to express arbitrary Groovy that
// Gradle would execute during its configuration phase (GHSA-3v79-m2cg-89ww). Permitting only
// a bare "name()" call - no ".", no arguments, no string-delimiter characters - means no
// method chain, command string or block break-out can be formed, while every documented
// repository shortcut (mavenCentral(), jcenter(), google(), mavenLocal(),
// gradlePluginPortal()) is accepted. Surrounding whitespace is trimmed before matching.
var repositoryPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*\(\)$`)

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
		return &newBuildAttributes, nil
	}

	// validate every user-supplied repository value before it is rendered into build.gradle,
	// to prevent Groovy/Gradle code injection during the build (GHSA-3v79-m2cg-89ww)
	for i, repository := range newBuildAttributes.Repositories {
		trimmedRepository := strings.TrimSpace(repository)
		if !repositoryPattern.MatchString(trimmedRepository) {
			return nil, errors.Errorf(
				"Invalid repository value %q: must be a no-argument repository declaration such as mavenCentral()",
				trimmedRepository)
		}
		newBuildAttributes.Repositories[i] = trimmedRepository
	}

	return &newBuildAttributes, nil
}
