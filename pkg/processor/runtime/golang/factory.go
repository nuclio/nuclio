package golang

import (
	"github.com/nuclio/nuclio-sdk/logger"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type factory struct{}

func (f *factory) Create(parentLogger logger.Logger,
	configuration *viper.Viper) (runtime.Runtime, error) {

	newConfiguration, err := runtime.NewConfiguration(configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create configuration")
	}

	return NewRuntime(parentLogger.GetChild("golang").(logger.Logger),
		&Configuration{
			Configuration:    *newConfiguration,
			EventHandlerName: configuration.GetString("name"),
		})
}

// register factory
func init() {
	runtime.RegistrySingleton.Register("golang", &factory{})
}
