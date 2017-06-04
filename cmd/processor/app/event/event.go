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
// Default implementation of event
//

type DefaultEvent struct {
	sourceInfoProvider SourceInfoProvider
	id                 ID
	emptyByteArray     []byte
	emptyHeaders       map[string]interface{}
	emptyTime          time.Time
}

func (de *DefaultEvent) GetVersion() int {
	return 0
}

func (de *DefaultEvent) SetSourceProvider(sourceInfoProvider SourceInfoProvider) {
	de.sourceInfoProvider = sourceInfoProvider
}

func (de *DefaultEvent) GetSource() SourceInfoProvider {
	return de.sourceInfoProvider
}

func (de *DefaultEvent) GetID() ID {
	return de.id
}

func (de *DefaultEvent) SetID(id ID) {
	de.id = id
}

func (de *DefaultEvent) GetContentType() string {
	return ""
}

func (de *DefaultEvent) GetBody() []byte {
	return de.emptyByteArray
}

func (de *DefaultEvent) GetSize() int {
	return 0
}

func (de *DefaultEvent) GetHeader(key string) []byte {
	return de.emptyByteArray
}

func (de *DefaultEvent) GetHeaderString(key string) string {
	return string(de.GetHeader(key))
}

func (de *DefaultEvent) GetHeaders() map[string]interface{} {
	return de.emptyHeaders
}

func (de *DefaultEvent) GetTimestamp() time.Time {
	return de.emptyTime
}
