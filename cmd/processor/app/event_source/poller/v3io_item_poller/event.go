package v3io_item_poller

import (
	"github.com/iguazio/v3io"

	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"time"
)

type Event struct {
	event.AbstractEvent
	item *v3io.ItemRespStruct
	url string
}

func (e *Event) GetHeader(key string) interface{} {
	return (*e.item)[key]
}

func (e *Event) GetHeaders() map[string]interface{} {
	return *e.item
}

func (e *Event) GetTimestamp() time.Time {
	secs := e.GetHeader("__mtime_secs").(int64)
	nsecs := e.GetHeader("__mtime_nsecs").(int64)

	return time.Unix(secs, nsecs)
}

func (e *Event) GetSize() int {
	return e.GetHeader("__size").(int)
}

func (e *Event) GetURL() string {
	return e.url
}
