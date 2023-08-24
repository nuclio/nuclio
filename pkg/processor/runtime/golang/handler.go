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

package golang

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

// entrypoint is the function which receives events
type entrypoint func(*nuclio.Context, nuclio.Event) (interface{}, error)

// context initializer is the function which is called per runtime to initialize context
type contextInitializer func(*nuclio.Context) error

type handler interface {

	// load will load a handler, given a runtime configuration
	load(*runtime.Configuration) error

	// getEntrypoint returns the entrypoint of the handler
	getEntrypoint() entrypoint

	// getContextInitializer returns the context initializer (if applicable) of the handler
	getContextInitializer() contextInitializer
}

type abstractHandler struct {
	logger             logger.Logger
	entrypoint         entrypoint
	contextInitializer contextInitializer
}

func (ah *abstractHandler) load(configuration *runtime.Configuration) error {

	// if configured, use the built-in handler
	if configuration.Spec.Build.Path == "nuclio:builtin" || configuration.Spec.Handler == "nuclio:builtin" {
		ah.logger.WarnWith("Using built in handler, as configured")

		ah.entrypoint = builtInHandler
		ah.contextInitializer = InitContext
	}

	return nil
}

// getEntrypoint returns the entrypoint of the handler
func (ah *abstractHandler) getEntrypoint() entrypoint {
	return ah.entrypoint
}

// getContextInitializer returns the context initializer (if applicable) of the handler
func (ah *abstractHandler) getContextInitializer() contextInitializer {
	return ah.contextInitializer
}

func (ah *abstractHandler) parseName(handlerName string) (string, string, error) {

	// if handler is empty, replace with default
	if handlerName == "" {
		handlerName = "main:Handler"
	}

	return functionconfig.ParseHandler(handlerName)
}
