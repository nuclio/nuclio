package python

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

	return NewRuntime(parentLogger.GetChild("python").(nuclio.Logger),
		&Configuration{
			Configuration: *newConfiguration,
			EntryPoint:    configuration.GetString("entry_point"),
		})
}

// register factory
func init() {
	runtime.RegistrySingleton.Register("python", &factory{})
}
