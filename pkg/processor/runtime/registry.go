package runtime

import (
	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/util/registry"

	"github.com/spf13/viper"
)

type Creator interface {
	Create(logger nuclio.Logger, configuration *viper.Viper) (Runtime, error)
}

type Registry struct {
	registry.Registry
}

// global singleton
var RegistrySingleton = Registry{
	Registry: *registry.NewRegistry("runtime"),
}

func (r *Registry) NewRuntime(logger nuclio.Logger,
	kind string,
	configuration *viper.Viper) (Runtime, error) {

	registree, err := r.Get(kind)
	if err != nil {
		return nil, err
	}

	return registree.(Creator).Create(logger, configuration)
}
