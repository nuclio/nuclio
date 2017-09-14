package python

import (
	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
)

type factory struct {}

func (f *factory) Create(logger nuclio.Logger, configuration runtime.Configuration) (runtime.Runtime, error) {
	return &python{
		AbstractRuntime: runtime.AbstractRuntime{
			Logger: logger,
			Configuration: configuration,
		},
	}, nil
}

func init() {
	runtime.RuntimeRegistrySingleton.Register("python", &factory{})
}
