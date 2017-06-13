package event_source

import (
	"sync"

	"github.com/spf13/viper"

	"errors"
	"fmt"
	"github.com/nuclio/nuclio/pkg/logger"
)

type Creator interface {
	Create(logger logger.Logger,
		eventSourceConfiguration *viper.Viper,
		runtimeConfiguration *viper.Viper) (EventSource, error)
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

func (esf *Registry) RegisterKind(kind string, creator Creator) {
	esf.lock.Lock()
	defer esf.lock.Unlock()

	esf.creatorByKind[kind] = creator
}

func (esf *Registry) NewEventSource(logger logger.Logger,
	eventSourceConfiguration *viper.Viper,
	runtimeConfiguration *viper.Viper) (EventSource, error) {
	esf.lock.Lock()
	defer esf.lock.Unlock()

	kind := eventSourceConfiguration.GetString("kind")

	// create by kind
	creator, found := esf.creatorByKind[kind]
	if !found {
		return nil, errors.New(fmt.Sprintf("Event source kind not supported. kind(%s)", kind))
	}

	// create by kind
	return creator.Create(logger,
		eventSourceConfiguration,
		runtimeConfiguration)
}
