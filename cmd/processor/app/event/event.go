package event

import (
	"time"
)

type SourceInfoProvider interface {

	// get the class of source (sync, async, etc)
	GetClass() string

	// get specific kind of source (http, rabbit mq, etc)
	GetKind() string
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

func (de *AbstractEvent) GetVersion() int {
	return 0
}

func (de *AbstractEvent) SetSourceProvider(sourceInfoProvider SourceInfoProvider) {
	de.sourceInfoProvider = sourceInfoProvider
}

func (de *AbstractEvent) GetSource() SourceInfoProvider {
	return de.sourceInfoProvider
}

func (de *AbstractEvent) GetID() ID {
	return de.id
}

func (de *AbstractEvent) SetID(id ID) {
	de.id = id
}

func (de *AbstractEvent) GetContentType() string {
	return ""
}

func (de *AbstractEvent) GetBody() []byte {
	return de.emptyByteArray
}

func (de *AbstractEvent) GetSize() int {
	return 0
}

func (de *AbstractEvent) GetHeader(key string) []byte {
	return de.emptyByteArray
}

func (de *AbstractEvent) GetHeaderString(key string) string {
	return string(de.GetHeader(key))
}

func (de *AbstractEvent) GetHeaders() map[string]interface{} {
	return de.emptyHeaders
}

func (de *AbstractEvent) GetTimestamp() time.Time {
	return de.emptyTime
}
