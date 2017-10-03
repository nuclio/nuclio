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

package functioncr

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/ghodss/yaml"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// allow alphanumeric (inc. underscore) and hyphen
var nameValidator = regexp.MustCompile(`^[\w\-]+$`).MatchString

type Function struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`
	Spec               FunctionSpec   `json:"spec"`
	Status             FunctionStatus `json:"status,omitempty"`
}

func (f *Function) SetDefaults() {
	f.TypeMeta.APIVersion = "nuclio.io/v1"
	f.TypeMeta.Kind = "Function"
	f.Namespace = "default"
}

func (f *Function) SetStatus(state FunctionState, message string) {
	f.Status.ObservedGen = f.ResourceVersion
	f.Status.State = state
	f.Status.Message = message
}

func (f *Function) GetLabels() map[string]string {
	if f.ObjectMeta.Labels == nil {
		f.ObjectMeta.Labels = make(map[string]string)
	}

	return f.Labels
}

func (f *Function) GetNameAndVersion() (name string, version *int, err error) {
	name = f.Name
	version = nil

	// verify name has only alphanumeric characters, underscores and hyphens
	if !nameValidator(f.Name) {
		err = errors.New("Name is invalid. Must only contain alphanumeric (inc. underscore) and hyphen")
		return
	}

	if lastHyphenIdx := strings.LastIndex(name, "-"); lastHyphenIdx > 0 {
		var versionValue int

		// get the string that follows the last hyphen
		versionValue, err = strconv.Atoi(name[lastHyphenIdx+1:])
		if err != nil {
			return
		}

		version = &versionValue
		name = name[:lastHyphenIdx]
	}

	return name, version, nil
}

func (f *Function) GetNamespacedName() string {
	return fmt.Sprintf("%s.%s", f.Namespace, f.Name)
}

func FromSpecFile(specFilePath string, f *Function) error {
	specFileContents, err := ioutil.ReadFile(specFilePath)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(specFileContents, f)
}
