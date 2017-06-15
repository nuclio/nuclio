package event

import (
	"time"
)

type SourceInfoProvider interface {

	// get the class of source (sync, async, etc)
	Class() string

	// get specific kind of source (http, rabbit mq, etc)
	Kind() string
}

//
// An event
//

type Event interface {
	GetVersion() int
	GetID() ID
	SetID(id ID)
	SetSourceProvider(sourceInfoProvider SourceInfoProvider)
	GetSource() SourceInfoProvider
	GetContentType() string
	GetBody() []byte
	GetSize() int
	GetHeader(key string) []byte
	GetHeaderString(key string) string
	GetHeaders() map[string]interface{}
	GetTimestamp() time.Time
}

//
// Abstract implementation of event
//

type AbstractEvent struct {
	sourceInfoProvider SourceInfoProvider
	id                 ID
	emptyByteArray     []byte
	emptyHeaders       map[string]interface{}
	emptyTime          time.Time
}

func (ae *AbstractEvent) GetVersion() int {
	return 0
}

func (ae *AbstractEvent) SetSourceProvider(sourceInfoProvider SourceInfoProvider) {
	ae.sourceInfoProvider = sourceInfoProvider
}

func (ae *AbstractEvent) GetSource() SourceInfoProvider {
	return ae.sourceInfoProvider
}

func (ae *AbstractEvent) GetID() ID {
	return ae.id
}

func (ae *AbstractEvent) SetID(id ID) {
	ae.id = id
}

func (ae *AbstractEvent) GetContentType() string {
	return ""
}

func (ae *AbstractEvent) GetBody() []byte {
	return ae.emptyByteArray
}

func (ae *AbstractEvent) GetSize() int {
	return 0
}

func (ae *AbstractEvent) GetHeader(key string) []byte {
	return ae.emptyByteArray
}

func (ae *AbstractEvent) GetHeaderString(key string) string {
	return string(ae.GetHeader(key))
}

func (ae *AbstractEvent) GetHeaders() map[string]interface{} {
	return ae.emptyHeaders
}

func (ae *AbstractEvent) GetTimestamp() time.Time {
	return ae.emptyTime
}
