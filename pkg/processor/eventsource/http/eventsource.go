package http

import (
	"errors"
	net_http "net/http"
	"time"

	"github.com/nuclio/nuclio/pkg/processor/eventsource"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	"github.com/nuclio/nuclio-sdk"

	"github.com/valyala/fasthttp"
)

type http struct {
	eventsource.AbstractEventSource
	configuration *Configuration
	event         Event
}

func newEventSource(logger nuclio.Logger,
	workerAllocator worker.WorkerAllocator,
	configuration *Configuration) (eventsource.EventSource, error) {

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

	// attach the context to the event
	h.event.ctx = ctx

	response, submitError, processError := h.SubmitEventToWorker(&h.event, 10*time.Second)

	// TODO: treat submit / process error differently?
	if submitError != nil || processError != nil {
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
	}
}
