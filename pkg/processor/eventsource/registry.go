/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package eventsource

import (
	"github.com/nuclio/nuclio/pkg/util/registry"

	"github.com/nuclio/nuclio-sdk"
	"github.com/spf13/viper"
)

type Creator interface {
	Create(logger nuclio.Logger,
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

func (r *Registry) NewEventSource(logger nuclio.Logger,
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
