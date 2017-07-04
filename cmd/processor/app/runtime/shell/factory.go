package shell

import (
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/nuclio/nuclio/cmd/processor/app/runtime"
	"github.com/nuclio/nuclio/pkg/logger"
)

type factory struct{}

func (f *factory) Create(logger logger.Logger,
	configuration *viper.Viper) (runtime.Runtime, error) {

	newConfiguration, err := runtime.NewConfiguration(configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create configuration")
	}

	return NewRuntime(logger.GetChild("shell"),
		&Configuration{
			Configuration: *newConfiguration,
			ScriptPath:    configuration.GetString("path"),
			ScriptArgs:    configuration.GetStringSlice("args"),
		})
}

// register factory
func init() {
	runtime.RegistrySingleton.Register("shell", &factory{})
}
