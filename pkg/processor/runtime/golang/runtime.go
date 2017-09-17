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
	nuclio "github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	golangruntimeeventhandler "github.com/nuclio/nuclio/pkg/processor/runtime/golang/event_handler"

	"github.com/nuclio/nuclio/pkg/errors"
)

type golang struct {
	*runtime.AbstractRuntime
	configuration *Configuration
	eventHandler  golangruntimeeventhandler.EventHandler
}

func NewRuntime(parentLogger nuclio.Logger, configuration *Configuration) (runtime.Runtime, error) {
	handlerName := configuration.EventHandlerName

	runtimeLogger := parentLogger.GetChild("golang").(nuclio.Logger)

	// if the handler name is not specified, just get the first one
	if handlerName == "" {
		eventKinds := golangruntimeeventhandler.EventHandlers.GetKinds()
		if len(eventKinds) == 0 {
			return nil, errors.New("No handlers registered, can't default to first")
		}

		handlerName = eventKinds[0]

		runtimeLogger.InfoWith("Handler name unspecified, using first", "handler", handlerName)
	}

	eventHandler, err := golangruntimeeventhandler.EventHandlers.Get(handlerName)
	if err != nil {
		return nil, err
	}

	// create base
	abstractRuntime, err := runtime.NewAbstractRuntime(runtimeLogger, &configuration.Configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract runtime")
	}

	// create the command string
	newGoRuntime := &golang{
		AbstractRuntime: abstractRuntime,
		configuration:   configuration,
		eventHandler:    eventHandler.(golangruntimeeventhandler.EventHandler),
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
