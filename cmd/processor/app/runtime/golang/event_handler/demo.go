package golang_runtime_event_handler

import (
	"github.com/nuclio/nuclio/cmd/processor/app/event"
	// "github.com/nuclio/nuclio/cmd/processor/app/event_source/http"
	"github.com/nuclio/nuclio/cmd/processor/app/event_source/poller/v3io_item_poller"
	"fmt"
)

func demo(event event.Event) (interface{}, error) {

	v3ioItem := event.(*v3io_item_poller.Event)
	fmt.Println(v3ioItem.GetURL())
	fmt.Println(v3ioItem.GetSize())
	fmt.Println(v3ioItem.GetTimestamp())

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
