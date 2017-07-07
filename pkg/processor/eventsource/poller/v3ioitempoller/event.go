package v3ioitempoller

import (
	"time"

	"github.com/nuclio/nuclio/pkg/processor/event"

	"github.com/iguazio/v3io"
)

type Event struct {
	event.AbstractEvent
	item *v3io.ItemRespStruct
	url  string
	path string
}

func (e *Event) GetHeader(key string) interface{} {
	return (*e.item)[key]
}

func (e *Event) GetHeaders() map[string]interface{} {
	return *e.item
}

func (e *Event) GetTimestamp() time.Time {
	secs := e.GetHeader("__mtime_secs").(int)
	nsecs := e.GetHeader("__mtime_nsecs").(int)

	return time.Unix(int64(secs), int64(nsecs))
}

func (e *Event) GetSize() int {
	return e.GetHeader("__size").(int)
}

func (e *Event) GetURL() string {
	return e.url
}

func (e *Event) GetPath() string {
	return e.path
}
