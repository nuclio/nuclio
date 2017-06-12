package http

import (
	"errors"
	"fmt"
	net_http "net/http"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/nuclio/nuclio/cmd/processor/app/event_source"
	"github.com/nuclio/nuclio/cmd/processor/app/worker"
	"github.com/nuclio/nuclio/pkg/logger"
)

type http struct {
	event_source.AbstractEventSource
	listenAddress string
	event         Event
}

func NewEventSource(logger logger.Logger,
	workerAllocator worker.WorkerAllocator,
	listenAddress string) (event_source.EventSource, error) {

	// we need a shareable allocator to support multiple go-routines. check that we were provided
	// with a valid allocator
	if !workerAllocator.Shareable() {
		return nil, errors.New("HTTP event source requires a shareable worker allocator")
	}

	newEventSource := http{
		AbstractEventSource: event_source.AbstractEventSource{
			Logger:          logger,
			WorkerAllocator: workerAllocator,
			Class:           "sync",
			Kind:            "http",
		},
		listenAddress: listenAddress,
		event:         Event{},
	}

	return &newEventSource, nil
}

func (h *http) Start(checkpoint event_source.Checkpoint) error {
	h.Logger.With(logger.Fields{
		"listenAddress": h.listenAddress,
	}).Info("Starting")

	// start listening
	go fasthttp.ListenAndServe(h.listenAddress, h.requestHandler)

	return nil
}

func (h *http) Stop(force bool) (event_source.Checkpoint, error) {

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
	case Response:

		// set body
		ctx.Response.SetBody(typedResponse.Body)

		// set headers
		for headerKey, headerValue := range typedResponse.Header {
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
		fmt.Fprint(ctx, string(typedResponse))
	}
}
