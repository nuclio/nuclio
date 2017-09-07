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
	"bytes"
	net_http "net/http"
	"time"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/eventsource"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/nuclio/pkg/zap"
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
)

type http struct {
	eventsource.AbstractEventSource
	configuration *Configuration
	event         Event
	bufferLogger  *nucliozap.NuclioZap
	bufferWriter  *bytes.Buffer
}

func newEventSource(logger nuclio.Logger,
	workerAllocator worker.WorkerAllocator,
	configuration *Configuration) (eventsource.EventSource, error) {

	bufferWriter := &bytes.Buffer{}

	// create a logger that is able to capture the output into a buffer. if a request arrives
	// and the user wishes to capture the log, this will be used as the logger instead of the default
	// logger
	bufferLogger, err := nucliozap.NewNuclioZap("function",
		"json",
		bufferWriter,
		bufferWriter,
		nucliozap.DebugLevel)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create buffer logger")
	}

	// we need a shareable allocator to support multiple go-routines. check that we were provided
	// with a valid allocator
	if !workerAllocator.Shareable() {
		return nil, errors.New("HTTP event source requires a shareable worker allocator")
	}

	newEventSource := http{
		AbstractEventSource: eventsource.AbstractEventSource{
			Logger:          logger,
			WorkerAllocator: workerAllocator,
			Class:           "sync",
			Kind:            "http",
		},
		configuration: configuration,
		event:         Event{},
		bufferLogger:  bufferLogger,
		bufferWriter:  bufferWriter,
	}

	return &newEventSource, nil
}

func (h *http) Start(checkpoint eventsource.Checkpoint) error {
	h.Logger.InfoWith("Starting", "listenAddress", h.configuration.ListenAddress)

	// start listening
	go fasthttp.ListenAndServe(h.configuration.ListenAddress, h.requestHandler)

	return nil
}

func (h *http) Stop(force bool) (eventsource.Checkpoint, error) {

	// TODO
	return nil, nil
}

func (h *http) requestHandler(ctx *fasthttp.RequestCtx) {
	var functionLogger nuclio.Logger
	var prevLevel nucliozap.Level

	// attach the context to the event
	h.event.ctx = ctx

	// get the log level required
	responseLogLevel := ctx.Request.Header.Peek("X-nuclio-log-level")

	// check if we need to return the logs as part of the response in the header
	if responseLogLevel != nil {

		// set the function logger to the runtime's logger capable of writing to a buffer
		// TODO: we should have a logger wrapper that can write to multiple loggers. this way function logs
		// get written both to the original function logger _and_ the HTTP stream
		functionLogger = h.bufferLogger

		// store previous level so we can restore it afterwards
		prevLevel = h.bufferLogger.GetLevel()

		// set the logger level
		h.bufferLogger.SetLevel(nucliozap.GetLevelByName(string(responseLogLevel)))

		// reset the buffer writer
		h.bufferWriter.Reset()
	}

	response, submitError, processError := h.SubmitEventToWorker(&h.event, functionLogger, 10*time.Second)

	if responseLogLevel != nil {
		logContents := h.bufferWriter.Bytes()
		ctx.Response.Header.SetBytesV("X-nuclio-logs", logContents[:len(logContents)-1])

		// restore the function logger level
		h.bufferLogger.SetLevel(prevLevel)
	}

	// if we failed to submit the event to a worker
	if submitError != nil {
		switch errors.Cause(submitError) {

		// no available workers
		case worker.ErrNoAvailableWorkers:
			ctx.Response.SetStatusCode(net_http.StatusServiceUnavailable)
			return

		// something else - most likely a bug
		default:
			h.Logger.WarnWith("Failed to submit event", "err", submitError)
			ctx.Response.SetStatusCode(net_http.StatusInternalServerError)
			return
		}
	}

	// if the function returned an error - just return 500
	if processError != nil {
		ctx.Response.SetStatusCode(net_http.StatusInternalServerError)
		return
	}

	// format the response into the context, based on its type
	switch typedResponse := response.(type) {
	case nuclio.Response:

		// set body
		ctx.Response.SetBody(typedResponse.Body)

		// set headers
		for headerKey, headerValue := range typedResponse.Headers {
			ctx.Response.Header.Set(headerKey, headerValue)
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
		ctx.Write(typedResponse)

	case string:
		ctx.WriteString(typedResponse)
	}
}
