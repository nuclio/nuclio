package shell

import (
	"github.com/spf13/viper"

	"github.com/nuclio/nuclio/cmd/processor/app/runtime"
	"github.com/nuclio/nuclio/pkg/logger"
)

type factory struct{}

func (f *factory) Create(logger logger.Logger,
	configuration *viper.Viper) (runtime.Runtime, error) {

	return NewRuntime(logger.GetChild("shell"),
		&Configuration{
			Configuration: *runtime.NewConfiguration(configuration),
			ScriptPath:    configuration.GetString("path"),
			ScriptArgs:    configuration.GetStringSlice("args"),
		})
}

// register factory
func init() {
	runtime.FactorySingleton.RegisterKind("shell", &factory{})
}
