package runtime

import (
	"github.com/spf13/viper"

	"github.com/nuclio/nuclio/pkg/logger"
	"github.com/nuclio/nuclio/pkg/util/registry"
)

type Creator interface {
	Create(logger logger.Logger, configuration *viper.Viper) (Runtime, error)
}

type Registry struct {
	registry.Registry
}

// global singleton
var RegistrySingleton = Registry{
	Registry: *registry.NewRegistry("runtime"),
}

func (r *Registry) NewRuntime(logger logger.Logger,
	kind string,
	configuration *viper.Viper) (Runtime, error) {

	registree, err := r.Get(kind)
	if err != nil {
		return nil, err
	}

	return registree.(Creator).Create(logger, configuration)
}
