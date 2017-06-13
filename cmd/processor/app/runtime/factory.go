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
	lock sync.Locker
	creatorByKind map[string]Creator
}

// global singleton
var FactorySingleton = Factory{
	lock: &sync.Mutex{},
	creatorByKind: map[string]Creator{},
}

func (rf *Factory) RegisterKind(kind string, creator Creator) {
	rf.lock.Lock()
	defer rf.lock.Unlock()

	rf.creatorByKind[kind] = creator
}

func (rf *Factory) Create(logger logger.Logger, configuration *viper.Viper) (Runtime, error) {
	rf.lock.Lock()
	defer rf.lock.Unlock()

	// create by kind
	return rf.creatorByKind[configuration.GetString("kind")].Create(logger, configuration)
}
