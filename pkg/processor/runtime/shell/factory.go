package shell

import (
	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type factory struct{}

func (f *factory) Create(parentLogger nuclio.Logger,
	configuration *viper.Viper) (runtime.Runtime, error) {

	newConfiguration, err := runtime.NewConfiguration(configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create configuration")
	}

	return NewRuntime(parentLogger.GetChild("shell").(nuclio.Logger),
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
