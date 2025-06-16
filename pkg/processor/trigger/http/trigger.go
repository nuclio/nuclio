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

package http

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	nethttp "net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/eventprocessor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/nuclio/errors"
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
	configuration      *Configuration
	events             []Event
	bufferLoggerPool   *nucliozap.BufferLoggerPool
	status             *status.SafeStatus
	server             *fasthttp.Server
	internalHealthPath []byte
}

func newTrigger(logger logger.Logger,
	workerAllocator eventprocessor.Allocator,
	configuration *Configuration,
	restartTriggerChan chan trigger.Trigger) (trigger.Trigger, error) {

	bufferLoggerPool, err := nucliozap.NewBufferLoggerPool(8,
		configuration.ID,
		"json",
		nucliozap.DebugLevel)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create buffer loggers")
	}

	numWorkers := len(workerAllocator.GetObjects())

	abstractTrigger, err := trigger.NewAbstractTrigger(logger,
		workerAllocator,
		&configuration.Configuration,
		"sync",
		"http",
		configuration.Name,
		restartTriggerChan)
	if err != nil {
		return nil, errors.New("Failed to create abstract trigger")
	}

	newTrigger := http{
		AbstractTrigger:    abstractTrigger,
		configuration:      configuration,
		bufferLoggerPool:   bufferLoggerPool,
		status:             status.NewSafeStatus(status.Initializing),
		internalHealthPath: []byte(InternalHealthPath),
	}

	newTrigger.AbstractTrigger.Trigger = &newTrigger
	newTrigger.allocateEvents(numWorkers)

	if functionconfig.BatchModeEnabled(configuration.Batch) {
		if batchTimeout, err := time.ParseDuration(configuration.Batch.Timeout); err != nil {
			return nil, errors.Errorf("Could not parse batch timeout: %s", configuration.Batch.Timeout)
		} else {
			workerAvailabilityTimeout := time.Duration(*newTrigger.configuration.WorkerAvailabilityTimeoutMilliseconds) * time.Millisecond
			go newTrigger.StartBatcher(batchTimeout, workerAvailabilityTimeout)
		}
		newTrigger.Logger.Debug("Batcher started")
	}
	return &newTrigger, nil
}

func (h *http) Start(checkpoint functionconfig.Checkpoint) error {
	h.Logger.InfoWith("Starting",
		"listenAddress", h.configuration.URL,
		"readBufferSize", h.configuration.ReadBufferSize,
		"maxRequestBodySize", h.configuration.MaxRequestBodySize,
		"reduceMemoryUsage", h.configuration.ReduceMemoryUsage,
		"cors", h.configuration.CORS)

	h.server = &fasthttp.Server{
		Handler:            h.onRequestFromFastHTTP(),
		Name:               "nuclio",
		ReadBufferSize:     h.configuration.ReadBufferSize,
		Logger:             NewFastHTTPLogger(h.Logger),
		MaxRequestBodySize: h.configuration.MaxRequestBodySize,
		ReduceMemoryUsage:  h.configuration.ReduceMemoryUsage,
	}

	// start listening
	go h.server.ListenAndServe(h.configuration.URL) // nolint: errcheck

	h.status.SetStatus(status.Ready)
	return nil
}

func (h *http) Stop(force bool) (functionconfig.Checkpoint, error) {
	h.Logger.Debug("Stopping HTTP trigger")

	h.status.SetStatus(status.Stopped)

	if h.server != nil {
		err := h.server.Shutdown()

		if err != nil {
			return nil, errors.Wrap(err, "Failed to stop server")
		}
	}

	return nil, nil
}

func (h *http) SignalWorkersToTerminate() error {
	h.status.SetStatus(status.Stopping)
	return h.AbstractTrigger.SignalWorkersToTerminate()
}

func (h *http) GetConfig() map[string]interface{} {
	return common.StructureToMap(h.configuration)
}

func (h *http) PrepareEventAndSubmitToBatch(ctx *fasthttp.RequestCtx) (chan interface{}, context.CancelFunc) {
	event := &Event{}
	event.ctx = ctx
	return h.SubmitEventToBatch(event)
}

func (h *http) AllocateWorkerAndSubmitEvent(ctx *fasthttp.RequestCtx,
	functionLogger logger.Logger,
	timeout time.Duration) (response nuclio.ProcessingResult, submitError error, processError error, workerInstance eventprocessor.EventProcessor) {

	defer h.HandleSubmitPanic(workerInstance, &submitError)

	// allocate a worker
	workerInstance, _, err := h.allocateWorker(timeout)
	if err != nil {
		return nil, err, nil, workerInstance
	}

	// submit to worker
	response, processError = h.SubmitEventToWorker(functionLogger, workerInstance, &Event{ctx: ctx})
	return response, nil, processError, workerInstance
}

func (h *http) flushIfTimeout(ctx *fasthttp.RequestCtx, processError error) bool {
	if processError != nil {
		if typedError, ok := processError.(nuclio.WithStatusCode); ok {
			if typedError.StatusCode() == nethttp.StatusRequestTimeout {
				// time out context
				ctx.SetStatusCode(nethttp.StatusRequestTimeout)
				bodyWrite := func(w *bufio.Writer) {
					w.Write(timeoutResponse) // nolint: errcheck
					w.Flush()                // nolint: errcheck
				}

				// This doesn't flush automatically, you still need to give fasthttp some
				// time to process
				ctx.SetBodyStreamWriter(bodyWrite)
				return true
			}
		}
	}
	return false
}

func (h *http) allocateWorker(timeout time.Duration) (eventprocessor.EventProcessor, int, error) {
	// allocate a worker
	workerInstance, err := h.WorkerAllocator.Allocate(timeout)
	if err != nil {
		h.UpdateStatistics(false, 1)
		return nil, -1, errors.Wrap(err, "Failed to allocate worker")
	}

	// use the event @ the worker index
	// TODO: event already used?
	workerIndex := workerInstance.GetIndex()
	if workerIndex < 0 || workerIndex >= len(h.events) {
		h.WorkerAllocator.Release(workerInstance)
		return nil, -1, errors.Errorf("Worker index (%d) bigger than size of event pool (%d)", workerIndex, len(h.events))
	}
	return workerInstance, workerIndex, nil
}

func (h *http) onRequestFromFastHTTP() fasthttp.RequestHandler {

	// when CORS is enabled, processor HTTP server is responding to "PreflightRequestMethod" (e.g.: OPTIONS)
	// That means => function will not be able to answer on the method configured by PreflightRequestMethod
	return func(ctx *fasthttp.RequestCtx) {

		// ensure request is part of CORS pre-flight
		if h.ensureRequestIsCORSPreflightRequest(ctx) {
			h.handlePreflightRequest(ctx)
		} else {
			h.handleRequest(ctx)
		}
	}
}

func (h *http) ensureRequestIsCORSPreflightRequest(ctx *fasthttp.RequestCtx) bool {
	if !h.configuration.corsEnabled() {

		// no cors is enabled, irrelevant
		return false
	}

	requestMethod := common.ByteSliceToString(ctx.Method())
	if requestMethod != h.configuration.CORS.PreflightRequestMethod {

		// request method is not preflight method
		return false
	}

	// access control request method must not be empty
	// access control describe the actual request method to be made (on the post pre-preflight request)
	accessControlRequestMethod := common.ByteSliceToString(ctx.Request.Header.Peek("Access-Control-Request-Method"))
	return len(accessControlRequestMethod) != 0
}

func (h *http) preflightRequestValidation(ctx *fasthttp.RequestCtx) bool {
	requestHeaders := &ctx.Request.Header
	accessControlRequestMethod := common.ByteSliceToString(requestHeaders.Peek("Access-Control-Request-Method"))
	accessControlRequestHeaders := common.ByteSliceToString(requestHeaders.Peek("Access-Control-Request-Headers"))

	// ensure origin is allowed
	origin := common.ByteSliceToString(requestHeaders.Peek("Origin"))
	corsConfiguration := h.configuration.CORS
	if !corsConfiguration.OriginAllowed(origin) {
		h.Logger.DebugWith("Origin is not allowed",
			"allowOrigins", h.configuration.CORS.AllowOrigins)
		return false
	}

	// request is outside the scope of CORS specifications
	if !corsConfiguration.MethodAllowed(accessControlRequestMethod) {
		h.Logger.DebugWith("Request method is not allowed",
			"allowMethods", h.configuration.CORS.AllowMethods)
		return false
	}

	// ensure request headers allowed, it is possible (and valid) to be empty
	if accessControlRequestHeaders != "" {
		if !corsConfiguration.HeadersAllowed(strings.Split(accessControlRequestHeaders, ", ")) {
			h.Logger.DebugWith("Request headers are not allowed",
				"accessControlRequestHeaders", accessControlRequestHeaders,
				"allowHeaders", h.configuration.CORS.AllowHeaders)
			return false
		}
	}
	return true
}

func (h *http) handlePreflightRequest(ctx *fasthttp.RequestCtx) {
	responseHeaders := &ctx.Response.Header
	corsConfiguration := h.configuration.CORS

	// default to bad preflight request unless all specifications are valid
	ctx.SetStatusCode(nethttp.StatusBadRequest)

	// always return vary <- https://stackoverflow.com/a/25329887
	responseHeaders.Add("Vary", "Origin")
	responseHeaders.Add("Vary", "Access-Control-Request-Method")
	responseHeaders.Add("Vary", "Access-Control-Request-Headers")

	// ensure preflight request headers are valid
	if !h.preflightRequestValidation(ctx) {
		h.UpdateStatistics(false, 1)
		return
	}

	// indicate whether resource can be shared
	origin := common.ByteSliceToString(ctx.Request.Header.Peek("Origin"))
	responseHeaders.Set("Access-Control-Allow-Origin", origin)

	// indicate resource support credentials
	if corsConfiguration.AllowCredentials {
		responseHeaders.Set("Access-Control-Allow-Credentials", corsConfiguration.EncodeAllowCredentialsHeader())
	}

	// set preflight results max age
	responseHeaders.Set("Access-Control-Max-Age", corsConfiguration.EncodePreflightMaxAgeSeconds())

	// indicate what methods can be used
	responseHeaders.Set("Access-Control-Allow-Methods", corsConfiguration.EncodedAllowMethods())

	// indicate what headers can be used
	responseHeaders.Set("Access-Control-Allow-Headers", corsConfiguration.EncodeAllowHeaders())

	// specifications met, set preflight request as OK
	ctx.SetStatusCode(nethttp.StatusOK)
	h.UpdateStatistics(true, 1)
}

func (h *http) preHandleRequestValidation(ctx *fasthttp.RequestCtx) bool {
	// Here, we want to allow not only the 'ready' status but also the 'stopping' status,
	// as it indicates that the HTTP service is about to stop. This allows it to process
	// requests during this time while also informing the readiness probe that the service
	// is no longer ready, preventing Kubernetes from sending further traffic to the pod.
	if ok := h.preHandleStatusValidation(ctx,
		status.Ready,
		status.Stopping); !ok {
		return false
	}
	// if cors is enabled, ensure request is valid
	if h.configuration.corsEnabled() {

		// always return vary <- https://stackoverflow.com/a/25329887
		ctx.Response.Header.Add("Vary", "Origin")

		// ensure origin is allowed
		origin := common.ByteSliceToString(ctx.Request.Header.Peek("Origin"))
		if !h.configuration.CORS.OriginAllowed(origin) {
			h.Logger.DebugWith("Origin is not allowed",
				"allowOrigins", h.configuration.CORS.AllowOrigins)
			return false
		}

		// indicate whether resource can be shared
		ctx.Response.Header.Set("Access-Control-Allow-Origin", origin)

		// indicate resource support credentials
		if h.configuration.CORS.AllowCredentials {
			ctx.Response.Header.Set("Access-Control-Allow-Credentials",
				h.configuration.CORS.EncodeAllowCredentialsHeader())
		}

		// set expose headers
		if len(h.configuration.CORS.ExposeHeaders) > 0 {
			ctx.Response.Header.Set("Access-Control-Expose-Headers",
				h.configuration.CORS.EncodeExposeHeaders())
		}
	}
	return true
}

func (h *http) preHandleStatusValidation(ctx *fasthttp.RequestCtx, expectedStatuses ...status.Status) bool {
	// Ensure the server is running
	if triggerStatus := h.status.GetStatus(); !triggerStatus.OneOf(expectedStatuses...) {
		h.Logger.DebugWith("Pre-handle validation failed because trigger is not ready",
			"triggerName", h.Name,
			"triggerKind", h.Kind,
			"triggerStatus", triggerStatus)
		ctx.Response.SetStatusCode(nethttp.StatusServiceUnavailable)
		msg := map[string]interface{}{
			"error":  "Server not ready",
			"status": h.status.String(),
		}

		if err := json.NewEncoder(ctx).Encode(msg); err != nil {
			h.Logger.WarnWith("Can't encode error message", "error", err)
		}
		return false
	}
	return true
}

func (h *http) handleRequest(ctx *fasthttp.RequestCtx) {
	var functionLogger logger.Logger
	var bufferLogger *nucliozap.BufferLogger

	// internal endpoint to allow clients the information whether the http server is taking requests in
	// this is an internal endpoint, we do not want to update statistics here
	if bytes.HasPrefix(ctx.URI().Path(), h.internalHealthPath) {
		// here we want to allow only ready status
		// because as soon as status has become non-ready,
		// we want k8s to stop sending traffic to this pod
		if ok := h.preHandleStatusValidation(
			ctx,
			status.Ready); !ok {
			return
		}
		ctx.Response.SetStatusCode(nethttp.StatusOK)
		return
	}

	// perform pre request handling validation
	if !h.preHandleRequestValidation(ctx) {

		// in case validation failed, stop here
		h.UpdateStatistics(false, 1)
		return
	}

	// attach the context to the event
	// get the log level required
	responseLogLevel := ctx.Request.Header.Peek(headers.LogLevel)

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
	var timedOut bool
	var response nuclio.ProcessingResult
	var submitError error
	var processError error
	var workerInstance eventprocessor.EventProcessor

	if functionconfig.BatchModeEnabled(h.configuration.Batch) {
		// cancelProcessing is a function that cancels the context to gracefully handle
		// channel closure and avoid potential deadlocks
		responseChan, cancelProcessing := h.PrepareEventAndSubmitToBatch(ctx)
		defer close(responseChan)

		// this flag indicates whether processing has been canceled
		var processingCancelled bool

		// wait for either event processing to finish or for the waiting timeout to pass
		select {
		case <-time.After(time.Duration(*h.configuration.WorkerAvailabilityTimeoutMilliseconds) * time.Millisecond):
			// timeout occurred, cancel event processing and set flags accordingly
			cancelProcessing()
			processingCancelled = true
			timedOut = true
			response = nil
			submitError = nil
			processError = nil
		case responseFromBatch := <-responseChan:
			// handle the response received from batch processing
			switch typedResponse := responseFromBatch.(type) {
			case *runtime.ResponseWithErrors:
				response = &typedResponse.Response
				submitError = typedResponse.SubmitError
				processError = typedResponse.ProcessError
			case nuclio.ProcessingResult:
				response = typedResponse
			}
		}
		// if event processing is not yet canceled, cancel it
		if !processingCancelled {
			cancelProcessing()
		}
	} else {
		// TODO: change to return runtime.ResponseWithErrors
		response, submitError, processError, workerInstance = h.AllocateWorkerAndSubmitEvent(ctx,
			functionLogger,
			time.Duration(*h.configuration.WorkerAvailabilityTimeoutMilliseconds)*time.Millisecond)
	}

	workerReleased := false
	errorDuringProcessing := processError != nil || submitError != nil || timedOut
	// if any error happened or response is not a stream, then release the worker instance asap
	if errorDuringProcessing ||
		response == nil || response != nil && !response.IsStream() {
		h.WorkerAllocator.Release(workerInstance)
		workerReleased = true
	} else {
		// in any other case (for now, only streaming case), defer the release of the worker instance
		defer func() {
			if !workerReleased {
				h.WorkerAllocator.Release(workerInstance)
			}
		}()
	}

	// if we got a process error, check if we timed out
	if processError != nil {
		timedOut = h.flushIfTimeout(ctx, processError)
	}

	if timedOut {
		return
	}

	h.setLogs(ctx, bufferLogger, responseLogLevel)

	// if we failed to submit the event to a worker
	if submitError != nil {
		switch errors.Cause(submitError) {

		// no available workers
		case eventprocessor.ErrNoAvailableObjects, eventprocessor.ErrAllObjectsAreTerminated:
			h.Logger.WarnWith("No workers available",
				"err", submitError.Error())
			ctx.Response.SetStatusCode(nethttp.StatusServiceUnavailable)

			// something else - most likely a bug
		default:
			h.Logger.WarnWith("Failed to submit event",
				"err", submitError.Error())
			ctx.Response.SetStatusCode(nethttp.StatusInternalServerError)
		}

		return
	}

	if processError != nil {
		var statusCode int

		// check if the user returned an error with a status code
		switch typedError := processError.(type) {
		case nuclio.WithStatusCode:
			statusCode = typedError.StatusCode()
		default:
			// if the user didn't use one of the errors with status code, return internal error
			statusCode = nethttp.StatusInternalServerError
		}

		ctx.Response.SetStatusCode(statusCode)
		ctx.Response.SetBodyString(processError.Error())
		return
	}

	fileStreamPath := ""
	fileStreamDeleteAfterSend := false

	// set headers
	for headerKey, headerValue := range response.GetHeaders() {

		// check if it's a special header
		if strings.EqualFold(headerKey, headers.FileStreamDeleteAfterSend) {
			fileStreamDeleteAfterSend = true
		} else {
			switch typedHeaderValue := headerValue.(type) {
			case string:
				if strings.EqualFold(headerKey, headers.FileStreamPath) {
					fileStreamPath = headerValue.(string)
				} else {
					ctx.Response.Header.Set(headerKey, typedHeaderValue)
				}
			case int:
				ctx.Response.Header.Set(headerKey, strconv.Itoa(typedHeaderValue))
			}
		}
	}

	if fileStreamPath != "" {
		fileResponse, err := newFileResponse(h.Logger, fileStreamPath, fileStreamDeleteAfterSend)
		if err != nil {
			if os.IsNotExist(err) {
				ctx.Response.SetStatusCode(nethttp.StatusNotFound)
			} else {
				h.Logger.WarnWith("Failed to open file for file streaming", "error", err)
				ctx.Response.SetStatusCode(nethttp.StatusInternalServerError)
			}

			return
		}
		ctx.Response.SetBodyStream(fileResponse, -1)
	} else {
		// set body
		switch typedResponse := response.GetBody().(type) {
		case []byte:
			ctx.Response.SetBodyRaw(response.GetBody().([]byte))
		case io.ReadCloser:
			if _, err := io.Copy(ctx.Response.BodyWriter(), typedResponse); err != nil {
				h.Logger.ErrorWith("Failed to copy response body",
					"error", err)
				ctx.Response.SetStatusCode(nethttp.StatusInternalServerError)
				return
			}
		}
	}

	// set content type if set
	if response.GetContentType() != "" {
		ctx.SetContentType(response.GetContentType())
	}

	// set status code if set
	if response.GetStatusCode() != 0 {
		ctx.Response.SetStatusCode(response.GetStatusCode())
	}
}

func (h *http) allocateEvents(size int) {
	h.events = make([]Event, size)
	for i := 0; i < size; i++ {
		h.events[i] = Event{}
	}
}

// setLogs attaches user code logs to the HTTP response header (if enabled).
// It finalizes the buffer logger output as a JSON array and sets it in the response header,
// unless the content size exceeds the header size limit (4096 bytes).
// This allows returning logs from the user code execution back to the client.
func (h *http) setLogs(ctx *fasthttp.RequestCtx, bufferLogger *nucliozap.BufferLogger, responseLogLevel []byte) {
	if responseLogLevel == nil {
		return
	}
	// remove trailing comma
	logContents := bufferLogger.Buffer.Bytes()

	// if there are no logs, we will only have the open bracket '[' we wrote before processing and we
	// want to keep that. so only remove the last character if there's more than the open bracket
	if len(logContents) > 1 {
		logContents = logContents[:len(logContents)-1]
	}

	// write close bracket for JSON
	logContents = append(logContents, byte(']'))

	// there's a limit on the amount of logs that can be passed in a header
	if len(logContents) < 4096 {
		ctx.Response.Header.SetBytesV(headers.Logs, logContents)
	} else {
		h.Logger.Warn("Skipped setting logs in header cause of size limit")
	}

	// return the buffer logger to the pool
	h.bufferLoggerPool.Release(bufferLogger)
}
