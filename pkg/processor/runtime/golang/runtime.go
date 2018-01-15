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
	"runtime/debug"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/nuclio-sdk"
)

type golang struct {
	*runtime.AbstractRuntime
	configuration *runtime.Configuration
	eventHandler  func(*nuclio.Context, nuclio.Event) (interface{}, error)
	loader        handlerLoader
}

func NewRuntime(parentLogger nuclio.Logger,
	configuration *runtime.Configuration,
	loader handlerLoader) (runtime.Runtime, error) {
	var err error

	runtimeLogger := parentLogger.GetChild("golang")

	// create base
	abstractRuntime, err := runtime.NewAbstractRuntime(runtimeLogger, configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract runtime")
	}

	// create the command string
	newGoRuntime := &golang{
		AbstractRuntime: abstractRuntime,
		configuration:   configuration,
		loader:          loader,
	}

	newGoRuntime.eventHandler, err = newGoRuntime.getHandlerFunc(configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get handler function")
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
	response, err = g.callEventHandler(event, functionLogger)

	// if a function logger was passed, restore previous
	if functionLogger != nil {
		g.Context.Logger = prevFunctionLogger
	}

	return response, err
}

func (g *golang) callEventHandler(event nuclio.Event, functionLogger nuclio.Logger) (response interface{}, responseErr error) {
	defer func() {
		if err := recover(); err != nil {
			callStack := debug.Stack()

			if functionLogger == nil {
				functionLogger = g.FunctionLogger
			}

			functionLogger.ErrorWith("Panic caught in event handler",
				"err",
				err,
				"stack",
				string(callStack))

			responseErr = fmt.Errorf("Caught panic: %s", err)
		}
	}()

	// before we call, save timestamp
	startTime := time.Now()

	response, responseErr = g.eventHandler(g.Context, event)

	// calculate how long it took to invoke the function
	callDuration := time.Since(startTime)

	// add duration to sum
	g.Statistics.DurationMilliSecondsSum += uint64(callDuration.Nanoseconds() / 1000000)
	g.Statistics.DurationMilliSecondsCount++

	return
}

// this is used for running a standalone processor during development
func (g *golang) builtInHandler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	return "Built in handler called", nil
}

func (g *golang) getHandlerFunc(configuration *runtime.Configuration) (func(*nuclio.Context, nuclio.Event) (interface{}, error), error) {
	var err error

	// if configured, use the built in handler
	if configuration.Spec.Build.Path == "nuclio:builtin" || configuration.Spec.Handler == "nuclio:builtin" {
		g.Logger.WarnWith("Using built in handler, as configured")

		return g.builtInHandler, nil
	}

	handlerName := configuration.Spec.Handler

	// if handler is empty, replace with default
	if handlerName == "" {
		handlerName = "main:Handler"
	}

	// parse the handler
	_, handlerName, err = g.parseHandler(handlerName)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse handler")
	}

	// try to load the handler function
	return g.loader.load(configuration.Spec.Build.Path, handlerName)
}

func (g *golang) parseHandler(handler string) (string, string, error) {

	// take the handler name, if a module was provided
	moduleAndEntrypoint := strings.Split(handler, ":")
	switch len(moduleAndEntrypoint) {

	// entrypoint only
	case 1:
		return "", moduleAndEntrypoint[0], nil

	// module:entrypoint
	case 2:
		return moduleAndEntrypoint[0], moduleAndEntrypoint[1], nil

	default:
		return "", "", fmt.Errorf("Invalid handler %s", handler)
	}
}
