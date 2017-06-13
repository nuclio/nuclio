package runtime

import (
	"sync"

	"github.com/spf13/viper"

	"errors"
	"fmt"
	"github.com/nuclio/nuclio/pkg/logger"
)

type Creator interface {
	Create(logger logger.Logger, configuration *viper.Viper) (Runtime, error)
}

type Registry struct {
	lock          sync.Locker
	creatorByKind map[string]Creator
}

// global singleton
var RegistrySingleton = Registry{
	lock:          &sync.Mutex{},
	creatorByKind: map[string]Creator{},
}

func (rf *Registry) RegisterKind(kind string, creator Creator) {
	rf.lock.Lock()
	defer rf.lock.Unlock()

	rf.creatorByKind[kind] = creator
}

func (rf *Registry) NewRuntime(logger logger.Logger, configuration *viper.Viper) (Runtime, error) {
	rf.lock.Lock()
	defer rf.lock.Unlock()

	kind := configuration.GetString("kind")

	// create by kind
	creator, found := rf.creatorByKind[kind]
	if !found {
		return nil, errors.New(fmt.Sprintf("Runtime kind not supported. kind(%s)", kind))
	}

	return creator.Create(logger, configuration)
}
