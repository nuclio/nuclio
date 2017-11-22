package main

import (
	"time"

	"github.com/nuclio/nuclio-sdk"
)

type SourceInfoProvider struct{}

func (si *SourceInfoProvider) GetClass() string {
	return "provider class"
}

func (si *SourceInfoProvider) GetKind() string {
	return "provider kind"
}

type Event struct {
	id      nuclio.ID
	headers map[string]interface{}
	fields  map[string]interface{}
}

func NewEvent() *Event {
	return &Event{
		id:      nuclio.NewID(),
		headers: map[string]interface{}{"h1": "hv1"},
		fields:  map[string]interface{}{"f1": "fv1"},
	}

}

func (evt *Event) GetVersion() int {
	return 7
}

func (evt *Event) GetID() nuclio.ID {
	return evt.id
}

func (evt *Event) SetID(id nuclio.ID) {
}

func (evt *Event) SetSourceProvider(sourceInfoProvider nuclio.SourceInfoProvider) {
}

func (evt *Event) GetSource() nuclio.SourceInfoProvider {
	return &SourceInfoProvider{}
}

func (evt *Event) GetContentType() string {
	return "content typee"
}

func (evt *Event) GetBody() []byte {
	return []byte("body")
}

func (evt *Event) GetSize() int {
	return len(evt.GetBody())
}

func (evt *Event) GetHeader(key string) interface{} {
	return evt.headers[key]
}

func (evt *Event) GetHeaderByteSlice(key string) []byte {
	return evt.headers[key].([]byte)
}

func (evt *Event) GetHeaderString(key string) string {
	return evt.headers[key].(string)
}

func (evt *Event) GetHeaders() map[string]interface{} {
	return evt.headers
}

func (evt *Event) GetField(key string) interface{} {
	return evt.fields[key]
}

func (evt *Event) GetFieldByteSlice(key string) []byte {
	return evt.headers[key].([]byte)
}

func (evt *Event) GetFieldString(key string) string {
	return evt.headers[key].(string)
}

func (evt *Event) GetFieldInt(key string) (int, error) {
	val, _ := evt.headers[key].(int)

	return val, nil
}

func (evt *Event) GetFields() map[string]interface{} {
	return evt.fields
}

func (evt *Event) GetTimestamp() time.Time {
	return time.Now()
}

func (evt *Event) GetPath() string {
	return "/path"
}

func (evt *Event) GetURL() string {
	return "https://example.com"
}

func (evt *Event) GetMethod() string {
	return "POST"
}
