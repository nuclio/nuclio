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
	"bytes"
	"encoding/json"
	nethttp "net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

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
	status             status.Status
	activeContexts     []*fasthttp.RequestCtx
	timeouts           []uint64 // flag of worker is in timeout
	answering          []uint64 // flag the worker is answering
	server             *fasthttp.Server
	internalHealthPath []byte
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
		"http",
		configuration.Name)
	if err != nil {
		return nil, errors.New("Failed to create abstract trigger")
	}

	newTrigger := http{
		AbstractTrigger:    abstractTrigger,
		configuration:      configuration,
		bufferLoggerPool:   bufferLoggerPool,
		status:             status.Initializing,
		activeContexts:     make([]*fasthttp.RequestCtx, numWorkers),
		timeouts:           make([]uint64, numWorkers),
		answering:          make([]uint64, numWorkers),
		internalHealthPath: []byte(InternalHealthPath),
	}

	newTrigger.allocateEvents(numWorkers)
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

	h.status = status.Ready
	return nil
}

func (h *http) Stop(force bool) (functionconfig.Checkpoint, error) {
	h.Logger.Debug("Shutting down")

	h.status = status.Stopped

	if h.server != nil {
		err := h.server.Shutdown()

		if err != nil {
			return nil, errors.Wrap(err, "Failed to stop server")
		}
	}

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

	ctx.SetStatusCode(nethttp.StatusRequestTimeout)
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
			"origin", origin,
			"allowOrigins", h.configuration.CORS.AllowOrigins)
		return false
	}

	// request is outside the scope of CORS specifications
	if !corsConfiguration.MethodAllowed(accessControlRequestMethod) {
		h.Logger.DebugWith("Request method is not allowed",
			"accessControlRequestMethod", accessControlRequestMethod,
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
		h.UpdateStatistics(false)
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
	h.UpdateStatistics(true)
}

func (h *http) preHandleRequestValidation(ctx *fasthttp.RequestCtx) bool {

	// ensure server is running
	if h.status != status.Ready {
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

	// if cors is enabled, ensure request is valid
	if h.configuration.corsEnabled() {

		// always return vary <- https://stackoverflow.com/a/25329887
		ctx.Response.Header.Add("Vary", "Origin")

		// ensure origin is allowed
		origin := common.ByteSliceToString(ctx.Request.Header.Peek("Origin"))
		if !h.configuration.CORS.OriginAllowed(origin) {
			h.Logger.DebugWith("Origin is not allowed",
				"origin", origin,
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

func (h *http) handleRequest(ctx *fasthttp.RequestCtx) {
	var functionLogger logger.Logger
	var bufferLogger *nucliozap.BufferLogger

	// perform pre request handling validation
	if !h.preHandleRequestValidation(ctx) {

		// in case validation failed, stop here
		h.UpdateStatistics(false)
		return
	}

	// internal endpoint to allow clients the information whether the http server is taking requests in
	// this is an internal endpoint, we do not want to update statistics here
	if bytes.HasPrefix(ctx.URI().Path(), h.internalHealthPath) {
		ctx.Response.SetStatusCode(nethttp.StatusOK)
		return
	}

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
		time.Duration(*h.configuration.WorkerAvailabilityTimeoutMilliseconds)*time.Millisecond)

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

		// write close bracket for JSON
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
			ctx.Response.SetStatusCode(nethttp.StatusServiceUnavailable)

			// something else - most likely a bug
		default:
			h.Logger.WarnWith("Failed to submit event", "err", submitError)
			ctx.Response.SetStatusCode(nethttp.StatusInternalServerError)
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
			statusCode = nethttp.StatusInternalServerError
		}

		ctx.Response.SetStatusCode(statusCode)
		ctx.Response.SetBodyString(processError.Error())
		return
	}

	// format the response into the context, based on its type
	switch typedResponse := response.(type) {
	case nuclio.Response:
		fileStreamPath := ""
		fileStreamDeleteAfterSend := false

		// set headers
		for headerKey, headerValue := range typedResponse.Headers {

			// check if it's a special header
			if strings.EqualFold(headerKey, "X-nuclio-filestream-delete-after-send") {
				fileStreamDeleteAfterSend = true
			} else {
				switch typedHeaderValue := headerValue.(type) {
				case string:
					if strings.EqualFold(headerKey, "X-nuclio-filestream-path") {
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
			ctx.Response.SetBodyRaw(typedResponse.Body)
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
		ctx.Response.SetBodyRaw(typedResponse)

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
