/*
Copyright 2021 The Nuclio Authors.

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

package logprocessing

import (
	"reflect"
	"strings"
)

type FunctionLogLine struct {
	Time    *string `json:"time"`
	Level   *string `json:"level"`
	Message *string `json:"message"`
	Name    *string `json:"name,omitempty"`
	More    *string `json:"more,omitempty"`

	// these fields may be filled by user function log lines
	Datetime *string           `json:"datetime"`
	With     map[string]string `json:"with,omitempty"`
}

// GetJSONFields returns FunctionLogLine json field names
func (f FunctionLogLine) GetJSONFields() []string {
	var jsonFields []string
	val := reflect.ValueOf(f)
	for i := 0; i < val.Type().NumField(); i++ {
		t := val.Type().Field(i)
		fieldName := t.Name

		switch jsonTag := t.Tag.Get("json"); jsonTag {
		case "-", "":
			jsonFields = append(jsonFields, fieldName)
		default:
			parts := strings.Split(jsonTag, ",")
			name := parts[0]
			if name == "" {
				name = fieldName
			}
			jsonFields = append(jsonFields, name)
		}
	}
	return jsonFields
}
