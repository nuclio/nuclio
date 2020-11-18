package shell

import (
	"time"

	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
)

const ResponseErrorFormat = "Failed to run shell command.\nError: %s\nOutput:%s"

type Configuration struct {
	*runtime.Configuration
	Arguments       string
	ResponseHeaders map[string]interface{}
	DefaultTimeout  *time.Duration
}

func NewConfiguration(runtimeConfiguration *runtime.Configuration) (*Configuration, error) {
	minute := time.Minute
	newConfiguration := Configuration{
		Configuration:  runtimeConfiguration,
		DefaultTimeout: &minute,
	}

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Spec.RuntimeAttributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	return &newConfiguration, nil
}
