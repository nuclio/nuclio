package config

import (
	"io"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"

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
	Function     function                        `json:"function"`
	Logger       logger                          `json:"logger"`
	DataBindings map[string]platform.DataBinding `json:"data_bindings"`
	Triggers     map[string]platform.Trigger     `json:"triggers"`
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
	dataBindings map[string]platform.DataBinding,
	triggers map[string]platform.Trigger) error {

	configuration := configuration{
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
	body, err := yaml.Marshal(&configuration)
	if err != nil {
		return errors.Wrap(err, "Failed to write configuration")
	}

	_, err = outputWriter.Write(body)
	return err
}
