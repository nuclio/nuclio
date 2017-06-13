package golang

import (
	"github.com/spf13/viper"

	"github.com/nuclio/nuclio/cmd/processor/app/runtime"
	"github.com/nuclio/nuclio/pkg/logger"
)

type factory struct{}

func (f *factory) Create(logger logger.Logger,
	configuration *viper.Viper) (runtime.Runtime, error) {

	return NewRuntime(logger.GetChild("golang"),
		&Configuration{
			Configuration:    *runtime.NewConfiguration(configuration),
			EventHandlerName: configuration.GetString("name"),
		})
}

// register factory
func init() {
	runtime.RegistrySingleton.RegisterKind("golang", &factory{})
}
