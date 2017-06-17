package golang

import (
	"github.com/spf13/viper"

	"github.com/nuclio/nuclio/cmd/processor/app/runtime"
	"github.com/nuclio/nuclio/pkg/logger"
)

type factory struct{}

func (f *factory) Create(logger logger.Logger,
	configuration *viper.Viper) (runtime.Runtime, error) {

	newConfiguration, err := runtime.NewConfiguration(configuration)
	if err != nil {
		return nil, logger.Report(err, "Failed to create configuration")
	}

	return NewRuntime(logger.GetChild("golang"),
		&Configuration{
			Configuration:    *newConfiguration,
			EventHandlerName: configuration.GetString("name"),
		})
}

// register factory
func init() {
	runtime.RegistrySingleton.Register("golang", &factory{})
}
