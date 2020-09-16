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
	"context"
	"encoding/json"
	"github.com/nuclio/nuclio-sdk-go"
	net_http "net/http"
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/status"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
)

type nethttp struct {
	trigger.AbstractTrigger
	configuration    *Configuration
	events           []NetEvent
	bufferLoggerPool *nucliozap.BufferLoggerPool
	status           status.Status
	activeContexts   []net_http.ResponseWriter
	timeouts         []uint64 // flag of worker is in timeout
	answering        []uint64 // flag the worker is answering
	server           *net_http.Server
}

func newNetTrigger(logger logger.Logger,
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
		"http",
		configuration.Name)
	if err != nil {
		return nil, errors.New("Failed to create abstract trigger")
	}

	newTrigger := nethttp{
		AbstractTrigger:  abstractTrigger,
		configuration:    configuration,
		bufferLoggerPool: bufferLoggerPool,
		status:           status.Initializing,
		activeContexts:   make([]net_http.ResponseWriter, numWorkers),
		timeouts:         make([]uint64, numWorkers),
		answering:        make([]uint64, numWorkers),
	}

	newTrigger.allocateEvents(numWorkers)
	return &newTrigger, nil
}

func (h *nethttp) Start(checkpoint functionconfig.Checkpoint) error {
	h.Logger.InfoWith("Starting",
		"listenAddress", h.configuration.URL,
		"readBufferSize", h.configuration.ReadBufferSize)

	h.server = &net_http.Server{
		Addr: h.configuration.URL,
		Handler: h,
	}

	// start listening
	go h.server.ListenAndServe() // nolint: errcheck

	h.status = status.Ready
	return nil
}

func (h *nethttp) Stop(force bool) (functionconfig.Checkpoint, error) {
	h.Logger.Debug("Shutting down")

	h.status = status.Stopped

	if h.server != nil {
		err := h.server.Shutdown(context.Background())

		if err != nil {
			return nil, errors.Wrap(err, "Failed to stop server")
		}
	}

	return nil, nil
}

func (h *nethttp) GetConfig() map[string]interface{} {
	return common.StructureToMap(h.configuration)
}

func (h *nethttp) TimeoutWorker(worker *worker.Worker) error {
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

	ctx.WriteHeader(net_http.StatusRequestTimeout)
	ctx.Write(timeoutResponse)
	return nil
}

func (h *nethttp) AllocateWorkerAndSubmitEvent(responseWriter net_http.ResponseWriter,
	request *net_http.Request,
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

	h.activeContexts[workerIndex] = responseWriter
	h.timeouts[workerIndex] = 0
	h.answering[workerIndex] = 0
	event := &h.events[workerIndex]
	event.request = request
	event.responseWriter = responseWriter

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

func (h *nethttp) ServeHTTP(responseWriter net_http.ResponseWriter, request *net_http.Request) {
	if h.status != status.Ready {
		h.UpdateStatistics(false)
		responseWriter.WriteHeader(net_http.StatusServiceUnavailable)
		msg := map[string]interface{}{
			"error":  "Server not ready",
			"status": h.status.String(),
		}

		if err := json.NewEncoder(responseWriter).Encode(msg); err != nil {
			h.Logger.WarnWith("Can't encode error message", "error", err)
		}
	}

	var functionLogger logger.Logger
	var bufferLogger *nucliozap.BufferLogger

	// attach the context to the event
	// get the log level required
	responseLogLevel := request.Header.Get("X-nuclio-log-level")

	// check if we need to return the logs as part of the response in the header
	if responseLogLevel != "" {

		// set the function logger to the runtime's logger capable of writing to a buffer
		bufferLogger, _ = h.bufferLoggerPool.Allocate(nil)

		// set the logger level
		bufferLogger.Logger.SetLevel(nucliozap.GetLevelByName(string(responseLogLevel)))

		// write open bracket for JSON
		bufferLogger.Buffer.Write([]byte("["))

		// set the function logger to that of the chosen buffer logger
		functionLogger, _ = nucliozap.NewMuxLogger(bufferLogger.Logger, h.Logger)
	}

	response, timedOut, submitError, processError := h.AllocateWorkerAndSubmitEvent(responseWriter,
		request,
		functionLogger,
		time.Duration(*h.configuration.WorkerAvailabilityTimeoutMilliseconds)*time.Millisecond)

	if timedOut {
		return
	}

	// Clear active context in case of error
	if submitError != nil || processError != nil {
		for i, activeCtx := range h.activeContexts {
			if activeCtx == responseWriter {
				h.activeContexts[i] = nil
				break
			}
		}
	}

	if responseLogLevel != "" {

		// remove trailing comma
		logContents := bufferLogger.Buffer.Bytes()

		// if there are no logs, we will only happen the open bracket [ we wrote above and we
		// want to keep that. so only remove the last character if there's more than the open bracket
		if len(logContents) > 1 {
			logContents = logContents[:len(logContents)-1]
		}

		// write close bracket for JSON
		logContents = append(logContents, byte(']'))

		// there's a limit on the amount of logs that can be passed in a header
		if len(logContents) < 4096 {
			responseWriter.Header().Set("X-nuclio-logs", string(logContents))
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
			responseWriter.WriteHeader(net_http.StatusServiceUnavailable)

			// something else - most likely a bug
		default:
			h.Logger.WarnWith("Failed to submit event", "err", submitError)
			responseWriter.WriteHeader(net_http.StatusInternalServerError)
		}

		return
	}

	if processError != nil {
		var statusCode int

		// check if the user returned an error with a status code
		switch typedError := processError.(type) {
		case nuclio.ErrorWithStatusCode:
			statusCode = typedError.StatusCode()
		case *nuclio.ErrorWithStatusCode:
			statusCode = typedError.StatusCode()
		default:

			// if the user didn't use one of the errors with status code, return internal error
			statusCode = net_http.StatusInternalServerError
		}

		responseWriter.WriteHeader(statusCode)
		responseWriter.Write([]byte(processError.Error()))
		return
	}

	// format the response into the context, based on its type
	switch typedResponse := response.(type) {
	case nuclio.Response:

		// set body
		responseWriter.Write(typedResponse.Body)

		// set headers
		for headerKey, headerValue := range typedResponse.Headers {
			switch typedHeaderValue := headerValue.(type) {
			case string:
				responseWriter.Header().Set(headerKey, typedHeaderValue)
			case int:
				responseWriter.Header().Set(headerKey, strconv.Itoa(typedHeaderValue))
			}
		}

		// set content type if set
		if typedResponse.ContentType != "" {
			responseWriter.Header().Set("Content-Type", typedResponse.ContentType)
		}

		// set status code if set
		if typedResponse.StatusCode != 0 {
			responseWriter.WriteHeader(typedResponse.StatusCode)
		}

	case []byte:
		responseWriter.Write(typedResponse) // nolint: errcheck

	case string:
		responseWriter.Write([]byte(typedResponse)) // nolint: errcheck
	}
}

func (h *nethttp) allocateEvents(size int) {
	h.events = make([]NetEvent, size)
	for i := 0; i < size; i++ {
		h.events[i] = NetEvent{}
	}
}
