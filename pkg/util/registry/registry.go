package registry

import (
	"errors"
	"fmt"
	"sync"
)

type Registry struct {
	className  string
	Lock       sync.Locker
	Registered map[string]interface{}
}

func NewRegistry(className string) *Registry {
	return &Registry{
		className:  className,
		Lock:       &sync.Mutex{},
		Registered: map[string]interface{}{},
	}
}

func (r *Registry) Register(kind string, registeree interface{}) {
	r.Lock.Lock()
	defer r.Lock.Unlock()

	_, found := r.Registered[kind]
	if found {

		// registries register things on package initialization; no place for error handling
		panic(fmt.Sprintf("Already registered: %s", kind))
	}

	r.Registered[kind] = registeree
}

func (r *Registry) Get(kind string) (interface{}, error) {
	r.Lock.Lock()
	defer r.Lock.Unlock()

	registree, found := r.Registered[kind]
	if !found {

		// registries register things on package initialization; no place for error handling
		return nil, errors.New(fmt.Sprintf("Registry for %s failed to find: %s", r.className, kind))
	}

	return registree, nil
}

func (r *Registry) GetKinds() []string {
	r.Lock.Lock()
	defer r.Lock.Unlock()

	keys := make([]string, 0, len(r.Registered))

	for key := range r.Registered {
		keys = append(keys, key)
	}

	return keys
}
