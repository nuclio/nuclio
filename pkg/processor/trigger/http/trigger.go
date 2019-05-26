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

package http

import (
	"bufio"
	"encoding/json"
	net_http "net/http"
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/status"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/zap"
	"github.com/valyala/fasthttp"
)


var (
	timeoutResponse = []byte(`{"error": "handler timed out"}`)
)

type http struct {
	trigger.AbstractTrigger
	configuration    *Configuration
	events           []Event
	bufferLoggerPool *nucliozap.BufferLoggerPool
	status           status.Status
	activeContexts   []*fasthttp.RequestCtx
	timeouts         []uint64 // flag of worker is in timeout
	answering        []uint64 // flag the worker is answering
}

func newTrigger(logger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {

	bufferLoggerPool, err := nucliozap.NewBufferLoggerPool(8,
		configuration.ID,
		"json",
		nucliozap.DebugLevel)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create buffer loggers")
	}

	// we need a shareable allocator to support multiple go-routines. check that we were provided
	// with a valid allocator
	if !workerAllocator.Shareable() {
		return nil, errors.New("HTTP trigger requires a shareable worker allocator")
	}

	numWorkers := len(workerAllocator.GetWorkers())

	abstractTrigger, err := trigger.NewAbstractTrigger(logger,
		workerAllocator,
		&configuration.Configuration,
		"sync",
		"http")
	if err != nil {
		return nil, errors.New("Failed to create abstract trigger")
	}

	newTrigger := http{
		AbstractTrigger:  abstractTrigger,
		configuration:    configuration,
		bufferLoggerPool: bufferLoggerPool,
		status:           status.Initializing,
		activeContexts:   make([]*fasthttp.RequestCtx, numWorkers),
		timeouts:         make([]uint64, numWorkers),
		answering:        make([]uint64, numWorkers),
	}

	newTrigger.allocateEvents(numWorkers)
	return &newTrigger, nil
}

func (h *http) Start(checkpoint functionconfig.Checkpoint) error {
	h.Logger.InfoWith("Starting",
		"listenAddress", h.configuration.URL,
		"readBufferSize", h.configuration.ReadBufferSize)

	s := &fasthttp.Server{
		Handler:        h.requestHandler,
		Name:           "nuclio",
		ReadBufferSize: h.configuration.ReadBufferSize,
	}

	// start listening
	go s.ListenAndServe(h.configuration.URL) // nolint: errcheck

	h.status = status.Ready
	return nil
}

func (h *http) Stop(force bool) (functionconfig.Checkpoint, error) {

	// TODO: Shutdown server (see https://github.com/valyala/fasthttp/issues/233)
	h.status = status.Stopped
	return nil, nil
}

func (h *http) GetConfig() map[string]interface{} {
	return common.StructureToMap(h.configuration)
}

func (h *http) TimeoutWorker(worker *worker.Worker) error {
	workerIndex := worker.GetIndex()
	if workerIndex < 0 || workerIndex >= len(h.activeContexts) {
		return errors.Errorf("Worker %d out of range", workerIndex)
	}

	h.timeouts[workerIndex] = 1
	time.Sleep(time.Millisecond) // Let worker do it's thing
	if h.answering[workerIndex] == 1 {
		return errors.Errorf("Worker %d answered the request", workerIndex)
	}

	ctx := h.activeContexts[workerIndex]
	if ctx == nil {
		return errors.Errorf("Worker %d answered the request", workerIndex)
	}

	h.activeContexts[workerIndex] = nil

	ctx.SetStatusCode(net_http.StatusRequestTimeout)
	bodyWrite := func(w *bufio.Writer) {
		w.Write(timeoutResponse) // nolint: errcheck
		w.Flush()                // nolint: errcheck
	}

	// This doesn't flush automatically, you still need to give fasthttp some
	// time to process
	ctx.SetBodyStreamWriter(bodyWrite)
	return nil
}

func (h *http) AllocateWorkerAndSubmitEvent(ctx *fasthttp.RequestCtx,
	functionLogger logger.Logger,
	timeout time.Duration) (response interface{}, timedOut bool, submitError error, processError error) {

	var workerInstance *worker.Worker

	defer h.HandleSubmitPanic(workerInstance, &submitError)

	// allocate a worker
	workerInstance, err := h.WorkerAllocator.Allocate(timeout)
	if err != nil {
		h.UpdateStatistics(false)
		return nil, false, errors.Wrap(err, "Failed to allocate worker"), nil
	}

	// use the event @ the worker index
	// TODO: event already used?
	workerIndex := workerInstance.GetIndex()
	if workerIndex < 0 || workerIndex >= len(h.events) {
		h.WorkerAllocator.Release(workerInstance)
		return nil, false, errors.Errorf("Worker index (%d) bigger than size of event pool (%d)", workerIndex, len(h.events)), nil
	}

	h.activeContexts[workerIndex] = ctx
	h.timeouts[workerIndex] = 0
	h.answering[workerIndex] = 0
	event := &h.events[workerIndex]
	event.ctx = ctx

	// submit to worker
	response, processError = h.SubmitEventToWorker(functionLogger, workerInstance, event)

	// release worker when we're done
	h.WorkerAllocator.Release(workerInstance)

	if h.timeouts[workerIndex] == 1 {
		return nil, true, nil, nil
	}

	h.answering[workerIndex] = 1
	h.activeContexts[workerIndex] = nil

	return response, false, nil, processError
}


func (h *http) requestHandler(ctx *fasthttp.RequestCtx) {
	if h.status != status.Ready {
		ctx.Response.SetStatusCode(net_http.StatusServiceUnavailable)
		msg := map[string]interface{}{
			"error":  "Server not ready",
			"status": h.status.String(),
		}

		if err := json.NewEncoder(ctx).Encode(msg); err != nil {
			h.Logger.WarnWith("Can't encode error message", "error", err)
		}
	}

	var functionLogger logger.Logger
	var bufferLogger *nucliozap.BufferLogger

	// attach the context to the event
	// get the log level required
	responseLogLevel := ctx.Request.Header.Peek("X-nuclio-log-level")

	// check if we need to return the logs as part of the response in the header
	if responseLogLevel != nil {

		// set the function logger to the runtime's logger capable of writing to a buffer
		bufferLogger, _ = h.bufferLoggerPool.Allocate(nil)

		// set the logger level
		bufferLogger.Logger.SetLevel(nucliozap.GetLevelByName(string(responseLogLevel)))

		// write open bracket for JSON
		bufferLogger.Buffer.Write([]byte("["))

		// set the function logger to that of the chosen buffer logger
		functionLogger, _ = nucliozap.NewMuxLogger(bufferLogger.Logger, h.Logger)
	}

	response, timedOut, submitError, processError := h.AllocateWorkerAndSubmitEvent(ctx,
		functionLogger,
		time.Duration(h.configuration.WorkerAvailabilityTimeoutMilliseconds)*time.Millisecond)

	if timedOut {
		return
	}

	// Clear active context in case of error
	if submitError != nil || processError != nil {
		for i, activeCtx := range h.activeContexts {
			if activeCtx == ctx {
				h.activeContexts[i] = nil
				break
			}
		}
	}

	if responseLogLevel != nil {

		// remove trailing comma
		logContents := bufferLogger.Buffer.Bytes()

		// if there are no logs, we will only happen the open bracket [ we wrote above and we
		// want to keep that. so only remove the last character if there's more than the open bracket
		if len(logContents) > 1 {
			logContents = logContents[:len(logContents)-1]
		}

		// write open bracket for JSON
		logContents = append(logContents, byte(']'))

		// there's a limit on the amount of logs that can be passed in a header
		if len(logContents) < 4096 {
			ctx.Response.Header.SetBytesV("X-nuclio-logs", logContents)
		} else {
			h.Logger.Warn("Skipped setting logs in header cause of size limit")
		}

		// return the buffer logger to the pool
		h.bufferLoggerPool.Release(bufferLogger)
	}

	// if we failed to submit the event to a worker
	if submitError != nil {
		switch errors.Cause(submitError) {

		// no available workers
		case worker.ErrNoAvailableWorkers:
			ctx.Response.SetStatusCode(net_http.StatusServiceUnavailable)

			// something else - most likely a bug
		default:
			h.Logger.WarnWith("Failed to submit event", "err", submitError)
			ctx.Response.SetStatusCode(net_http.StatusInternalServerError)
		}

		return
	}

	// if the function returned an error - just return 500
	if processError != nil {
		var statusCode int

		// check if the user returned an error with a status code
		errorWithStatusCode, errorHasStatusCode := processError.(nuclio.ErrorWithStatusCode)

		// if the user didn't use one of the errors with status code, return internal error
		// otherwise return the status code the user wanted
		if !errorHasStatusCode {
			statusCode = net_http.StatusInternalServerError
		} else {
			statusCode = errorWithStatusCode.StatusCode()
		}

		ctx.Response.SetStatusCode(statusCode)
		return
	}

	// format the response into the context, based on its type
	switch typedResponse := response.(type) {
	case nuclio.Response:

		// set body
		ctx.Response.SetBody(typedResponse.Body)

		// set headers
		for headerKey, headerValue := range typedResponse.Headers {
			switch typedHeaderValue := headerValue.(type) {
			case string:
				ctx.Response.Header.Set(headerKey, typedHeaderValue)
			case int:
				ctx.Response.Header.Set(headerKey, strconv.Itoa(typedHeaderValue))
			}
		}

		// set content type if set
		if typedResponse.ContentType != "" {
			ctx.SetContentType(typedResponse.ContentType)
		}

		// set status code if set
		if typedResponse.StatusCode != 0 {
			ctx.Response.SetStatusCode(typedResponse.StatusCode)
		}

	case []byte:
		ctx.Write(typedResponse) // nolint: errcheck

	case string:
		ctx.WriteString(typedResponse) // nolint: errcheck
	}
}

func (h *http) allocateEvents(size int) {
	h.events = make([]Event, size)
	for i := 0; i < size; i++ {
		h.events[i] = Event{}
	}
}
