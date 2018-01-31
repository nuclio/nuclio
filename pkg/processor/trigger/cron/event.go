package cron

import (
	"github.com/nuclio/nuclio-sdk-go"
)

type Event struct {
	nuclio.AbstractEvent
	body    []byte
	headers map[string]interface{}
}

func (e *Event) GetBody() []byte {
	return e.body
}

func (e *Event) GetHeader(key string) interface{} {
	return e.headers[key]
}

func (e *Event) GetHeaderByteSlice(key string) []byte {
	return e.headers[key].([]byte)
}

func (e *Event) GetHeaderString(key string) string {
	return e.headers[key].(string)
}

func (e *Event) GetHeaders() map[string]interface{} {
	return e.headers
}
