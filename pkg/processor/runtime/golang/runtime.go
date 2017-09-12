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
	"fmt"
	"plugin"

	nuclio "github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/pkg/errors"
)

type golang struct {
	*runtime.AbstractRuntime
	configuration *Configuration
	eventHandler  nuclio.EventHandler
}

// NewRuntime return a new Go runtime
func NewRuntime(parentLogger nuclio.Logger, configuration *Configuration) (runtime.Runtime, error) {

	runtimeLogger := parentLogger.GetChild("golang").(nuclio.Logger)
	// create base
	abstractRuntime, err := runtime.NewAbstractRuntime(runtimeLogger, &configuration.Configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract runtime")
	}

	// create the command string
	newGoRuntime := &golang{
		AbstractRuntime: abstractRuntime,
		configuration:   configuration,
	}
	if err := newGoRuntime.loadEventHandler(); err != nil {
		return nil, err
	}

	return newGoRuntime, nil
}

func (g *golang) ProcessEvent(event nuclio.Event, functionLogger nuclio.Logger) (response interface{}, err error) {
	var prevFunctionLogger nuclio.Logger

	// if a function logger was passed, override the existing
	if functionLogger != nil {
		prevFunctionLogger = g.Context.Logger
		g.Context.Logger = functionLogger
	}

	// call the registered event handler
	response, err = g.eventHandler(g.Context, event)

	// if a function logger was passed, restore previous
	if functionLogger != nil {
		g.Context.Logger = prevFunctionLogger
	}

	return response, err
}

func (g *golang) loadEventHandler() error {
	dllPath := g.configuration.EventHandlerDLLPath
	plug, err := plugin.Open(dllPath)
	if err != nil {
		return errors.Wrapf(err, "Can't load DLL at %q", dllPath)
	}

	handlerName := g.configuration.EventHandlerName
	sym, err := plug.Lookup(handlerName)
	if err != nil {
		return errors.Wrapf(err, "Can't find handler %q in %q", handlerName, dllPath)
	}

	// TODO: Find out why "sym.(nuclio.EventHandler)" doesn't work
	eventHandler, ok := sym.(func(*nuclio.Context, nuclio.Event) (interface{}, error))
	if !ok {
		return fmt.Errorf("Wrong type (%T) for %s:%s", sym, dllPath, handlerName)
	}

	g.eventHandler = eventHandler
	return nil
}
