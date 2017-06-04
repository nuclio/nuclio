package golang_runtime_event_handler

import (
	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"github.com/nuclio/nuclio/cmd/processor/app/event_source/http"
)

func demo(event event.Event) (interface{}, error) {
	return http.Response{
		StatusCode:  201,
		ContentType: "application/text",
		Header: map[string]string{
			"x-v3io-something": "30",
			// "x-v3io-request-id": string(*event.GetID()),
		},
		Body: []byte("Response from golang"),
	}, nil
}
