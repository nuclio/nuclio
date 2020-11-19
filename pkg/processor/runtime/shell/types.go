package shell

import (
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
)

const ResponseErrorFormat = "Failed to run shell command.\nError: %s\nOutput:%s"

type Configuration struct {
	*runtime.Configuration
	Arguments       string
	ResponseHeaders map[string]interface{}
}

func NewConfiguration(runtimeConfiguration *runtime.Configuration) (*Configuration, error) {
	newConfiguration := Configuration{
		Configuration: runtimeConfiguration,
	}

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Spec.RuntimeAttributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	return &newConfiguration, nil
}
