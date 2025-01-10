/*
Copyright 2023 The Nuclio Authors.

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
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type golang struct {
	*runtime.AbstractRuntime
	configuration *runtime.Configuration
	entrypoint    entrypoint

	// TODO: support multiple cancels when implementing async
	cancelEventHandlingChan chan context.CancelFunc
}

// NewRuntime returns a new golang runtime
func NewRuntime(parentLogger logger.Logger,
	configuration *runtime.Configuration,
	handler handler) (runtime.Runtime, error) {
	var err error

	runtimeLogger := parentLogger.GetChild("golang")

	// create base
	abstractRuntime, err := runtime.NewAbstractRuntime(runtimeLogger, configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract runtime")
	}

	// load the handler
	if err := handler.load(configuration); err != nil {
		return nil, errors.Wrap(err, "Failed to load handler")
	}

	// create the runtime
	newGoRuntime := &golang{
		AbstractRuntime: abstractRuntime,
		configuration:   configuration,
		entrypoint:      handler.getEntrypoint(),
	}

	// try to initialize the context, if applicable
	contextInitializer := handler.getContextInitializer()
	if contextInitializer != nil {
		newGoRuntime.AbstractRuntime.Logger.DebugWith("Calling context initializer")

		if err := contextInitializer(newGoRuntime.Context); err != nil {
			return nil, errors.Wrap(err, "Failed to initialize context")
		}
	}
	newGoRuntime.cancelEventHandlingChan = make(chan context.CancelFunc, 1)
	newGoRuntime.SetStatus(status.Ready)

	return newGoRuntime, nil
}

func (g *golang) ProcessEvent(event nuclio.Event, functionLogger logger.Logger) (response interface{}, err error) {
	var prevFunctionLogger logger.Logger

	// if a function logger was passed, override the existing
	if functionLogger != nil {
		prevFunctionLogger = g.Context.Logger
		g.Context.Logger = functionLogger
	}

	// call the registered entrypoint
	response, err = g.callEntrypoint(event, functionLogger)

	// if a function logger was passed, restore previous
	if functionLogger != nil {
		g.Context.Logger = prevFunctionLogger
	}

	return response, err
}

func (g *golang) ProcessBatch(batch []nuclio.Event, functionLogger logger.Logger) ([]*runtime.ResponseWithErrors, error) {
	return nil, nuclio.ErrNotImplemented
}

func (g *golang) Restart() error {
	if err := g.Stop(); err != nil {
		return errors.Wrap(err, "Failed to stop golang runtime")
	}
	g.SetStatus(status.Ready)
	return nil
}

func (g *golang) Stop() error {
	g.SetStatus(status.Stopped)

	select {
	case cancelEventHandling := <-g.cancelEventHandlingChan:
		cancelEventHandling()
	default:
	}

	return nil
}

// SupportsRestart returns true if the runtime supports restart
func (g *golang) SupportsRestart() bool {
	return true
}

func (g *golang) callEntrypoint(event nuclio.Event, functionLogger logger.Logger) (interface{}, error) {
	if currentStatus := g.GetStatus(); currentStatus != status.Ready {
		return nil, errors.Errorf("Runtime not ready (current status: %s)", currentStatus)
	}
	defer func() {
		if err := recover(); err != nil {
			callStack := debug.Stack()

			if functionLogger == nil {
				functionLogger = g.FunctionLogger
			}

			functionLogger.ErrorWith("Panic caught in event handler",
				"err", err,
				"stack", string(callStack))
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	g.cancelEventHandlingChan <- cancel
	responseChan := make(chan *processingResponse)

	// Run the function in a goroutine
	go func() {
		processingResult := &processingResponse{}
		defer close(responseChan)
		defer func() {
			if err := recover(); err != nil {
				callStack := debug.Stack()

				if functionLogger == nil {
					functionLogger = g.FunctionLogger
				}

				functionLogger.ErrorWith("Panic caught in event handler",
					"err", err,
					"stack", string(callStack))

				processingResult.responseErr = fmt.Errorf("Caught panic: %s", err)
				// try to write response to the channel if it wasn't yet
				select {
				// if the reader is waiting, then it means that runtime wasn't stopped and waits for a response
				case responseChan <- processingResult:
				default:
				}
			}
		}()
		// before we call, save timestamp
		startTime := time.Now()

		// call entrypoint
		processingResult.response, processingResult.responseErr = g.entrypoint(g.Context, event)

		// calculate how long it took to invoke the function
		callDuration := time.Since(startTime)

		select {
		// if the reader is waiting, then it means that runtime wasn't stopped and waits for a response
		case responseChan <- processingResult:
			// add duration to sum
			g.Statistics.DurationMilliSecondsSum += uint64(callDuration.Nanoseconds() / 1000000)
			g.Statistics.DurationMilliSecondsCount++
		default:
		}
	}()

	select {
	case result := <-responseChan:
		select {
		case cancelEventHandling := <-g.cancelEventHandlingChan:
			// cancelling cancel-context
			defer cancelEventHandling()
		default:
		}
		return result.response, result.responseErr
	case <-ctx.Done():
		return nil, errors.New("Event processing was cancelled")
	}
}

type processingResponse struct {
	response    interface{}
	responseErr error
}
