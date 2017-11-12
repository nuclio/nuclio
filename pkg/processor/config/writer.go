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

package config

import (
	"io"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/ghodss/yaml"
)

type function struct {
	Handler string `json:"handler"`
	Runtime string `json:"runtime"`
}

type logger struct {
	Level string `json:"level"`
}

type configuration struct {
	Function     function                              `json:"function"`
	Logger       logger                                `json:"logger"`
	DataBindings map[string]functionconfig.DataBinding `json:"dataBindings"`
	Triggers     map[string]functionconfig.Trigger     `json:"triggers"`
}

type Writer struct{}

// NewWriter creates a writer
func NewWriter() *Writer {
	return &Writer{}
}

// Write writes a YAML file to the provided writer, based on all the arguments passed
func (w *Writer) Write(outputWriter io.Writer,
	handler string,
	runtime string,
	logLevel string,
	dataBindings map[string]functionconfig.DataBinding,
	triggers map[string]functionconfig.Trigger) error {

	functionConfiguration := configuration{
		Function: function{
			Handler: handler,
			Runtime: runtime,
		},
		Logger: logger{
			Level: logLevel,
		},
		DataBindings: dataBindings,
		Triggers:     triggers,
	}

	// write
	body, err := yaml.Marshal(&functionConfiguration)
	if err != nil {
		return errors.Wrap(err, "Failed to write configuration")
	}

	_, err = outputWriter.Write(body)
	return err
}
