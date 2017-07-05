package golangruntimeeventhandler

import (
	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"github.com/nuclio/nuclio/cmd/processor/app/runtime"

	"github.com/pkg/errors"
)

func demo(context *runtime.Context, event event.Event) (interface{}, error) {

	// get the full data of the object
	itemContents, err := context.V3ioClient.Get(event.GetPath())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get item contents")
	}

	context.Logger.DebugWith("Processing event in demo",
		"url", event.GetURL(),
		"size", event.GetSize(),
		"timestamp", event.GetTimestamp(),
		"contents", string(itemContents))

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
