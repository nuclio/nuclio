package golang_runtime_event_handler

import (
	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"github.com/nuclio/nuclio/cmd/processor/app/event_source/poller/v3io_item_poller"
	"github.com/nuclio/nuclio/cmd/processor/app/runtime"
	"github.com/nuclio/nuclio/pkg/logger"
)

func demo(context *runtime.Context, event event.Event) (interface{}, error) {

	v3ioItem := event.(*v3io_item_poller.Event)

	// get the full data of the object
	itemContents, err := context.V3ioClient.Get(v3ioItem.GetPath())
	if err != nil {
		return nil, context.Logger.Report(err, "Failed to get item contents")
	}

	context.Logger.With(logger.Fields{
		"url":       v3ioItem.GetURL(),
		"size":      v3ioItem.GetSize(),
		"timestamp": v3ioItem.GetTimestamp(),
		"contents": string(itemContents),
	}).Debug("Got event")

	return nil, nil

	//return http.Response{
	//	StatusCode:  201,
	//	ContentType: "application/text",
	//	Header: map[string]string{
	//		"x-v3io-something": "30",
	//		// "x-v3io-request-id": string(*event.GetID()),
	//	},
	//	Body: []byte("Response from golang"),
	//}, nil
}

func init() {
	EventHandlers.Add("demo", demo)
}
