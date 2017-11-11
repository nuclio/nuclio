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
	"runtime/debug"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	nuclio "github.com/nuclio/nuclio-sdk"
)

type golang struct {
	*runtime.AbstractRuntime
	configuration *Configuration
	eventHandler  func(*nuclio.Context, nuclio.Event) (interface{}, error)
}

// NewRuntime returns a new Go runtime
func NewRuntime(parentLogger nuclio.Logger, configuration *Configuration) (runtime.Runtime, error) {
	runtimeLogger := parentLogger.GetChild("golang")

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

	if err := newGoRuntime.loadHandler(); err != nil {
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

	// call the event handler
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

func (g *golang) loadHandler() error {
	dll, err := plugin.Open(g.configuration.DLLPath)
	if err != nil {
		return errors.Wrapf(err, "Can't load plugin at %q", g.configuration.DLLPath)
	}

	sym, err := dll.Lookup(g.configuration.EventHandlerName)
	if err != nil {
		return errors.Wrapf(err, "Can't find handler %q in %q", g.configuration.EventHandlerName, g.configuration.DLLPath)
	}

	handler, ok := sym.(func(*nuclio.Context, nuclio.Event) (interface{}, error))
	if !ok {
		return fmt.Errorf("%s:%s is from wrong type - %T", g.configuration.DLLPath, g.configuration.EventHandlerName, sym)
	}

	g.eventHandler = handler

	return nil
}
