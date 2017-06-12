package runtime

import (
	"sync"

	"github.com/spf13/viper"

	"github.com/nuclio/nuclio/pkg/logger"
)

type Creator interface {
	Create(logger logger.Logger, configuration *viper.Viper) (Runtime, error)
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

func (esf *Factory) Create(logger logger.Logger, configuration *viper.Viper) (Runtime, error) {

	// create by kind
	return esf.creatorByKind[configuration.GetString("kind")].Create(logger, configuration)
}
