package shell

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/mitchellh/mapstructure"
)

type Configuration struct {
	*runtime.Configuration
	ResponseHeaders map[string]interface{}
}

func NewConfiguration(runtimeConfiguration *runtime.Configuration) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	newConfiguration.Configuration = &runtime.Configuration{
		Configuration: runtimeConfiguration.Configuration,
	}

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Spec.RuntimeAttributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	return &newConfiguration, nil
}
