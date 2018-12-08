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

package loggersink

import (
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/registry"

	"github.com/nuclio/logger"
)

// Creator creates a logger sink instance
type Creator interface {

	// Create creates a logger sink instance
	Create(string, *platformconfig.LoggerSinkWithLevel) (logger.Logger, error)
}

type Registry struct {
	registry.Registry
}

// global singleton
var RegistrySingleton = Registry{
	Registry: *registry.NewRegistry("loggersink"),
}

// NewLoggerSink creates a new logger sink
func (r *Registry) NewLoggerSink(kind string,
	name string,
	loggerSinkConfiguration *platformconfig.LoggerSinkWithLevel) (logger.Logger, error) {

	registree, err := r.Get(kind)
	if err != nil {
		return nil, err
	}

	return registree.(Creator).Create(name, loggerSinkConfiguration)
}
