package eventsource

import (
	"github.com/nuclio/nuclio/pkg/logger"
	"github.com/nuclio/nuclio/pkg/util/registry"

	"github.com/spf13/viper"
)

type Creator interface {
	Create(logger logger.Logger,
		eventSourceConfiguration *viper.Viper,
		runtimeConfiguration *viper.Viper) (EventSource, error)
}

type Registry struct {
	registry.Registry
}

// global singleton
var RegistrySingleton = Registry{
	Registry: *registry.NewRegistry("event_source"),
}

func (r *Registry) NewEventSource(logger logger.Logger,
	kind string,
	eventSourceConfiguration *viper.Viper,
	runtimeConfiguration *viper.Viper) (EventSource, error) {

	registree, err := r.Get(kind)
	if err != nil {
		return nil, err
	}

	return registree.(Creator).Create(logger,
		eventSourceConfiguration,
		runtimeConfiguration)
}
