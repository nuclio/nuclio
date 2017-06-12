package event_source

import (
	"sync"

	"github.com/spf13/viper"

	"github.com/nuclio/nuclio/pkg/logger"
)

type Creator interface {
	Create(logger logger.Logger,
		eventSourceConfiguration *viper.Viper,
		runtimeConfiguration *viper.Viper) (EventSource, error)
}

type Factory struct {
	lock sync.Mutex
	creatorByKind map[string]Creator
}

// global singleton
var FactorySingleton = Factory{
	creatorByKind: map[string]Creator{},
}

func (esf *Factory) RegisterKind(kind string, creator Creator) {
	esf.creatorByKind[kind] = creator
}

func (esf *Factory) Create(logger logger.Logger,
	eventSourceConfiguration *viper.Viper,
	runtimeConfiguration *viper.Viper) (EventSource, error) {

	// create by kind
	return esf.creatorByKind[eventSourceConfiguration.GetString("kind")].Create(logger,
		eventSourceConfiguration,
		runtimeConfiguration)
}
