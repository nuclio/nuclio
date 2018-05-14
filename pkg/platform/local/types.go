package local

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/mitchellh/mapstructure"
)

type functionPlatformConfiguration struct {
	Network string
}

func newFunctionPlatformConfiguration(functionConfig *functionconfig.Config) (*functionPlatformConfiguration, error) {
	newConfiguration := functionPlatformConfiguration{}

	// parse attributes
	if err := mapstructure.Decode(functionConfig.Spec.Platform.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	return &newConfiguration, nil
}
