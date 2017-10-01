package nuclio

import (
	"time"
)

// SourceInfoProvider provides information about the source provider
type SourceInfoProvider interface {
	// GetClass returns the class of source (sync, async, etc)
	GetClass() string
	// GetKind returns the specific kind of source (http, rabbit mq, etc)
	GetKind() string
}

// Event is a nuclio event
type Event interface {
	GetVersion() int
	GetID() ID
	SetID(id ID)
	SetSourceProvider(sourceInfoProvider SourceInfoProvider)
	GetSource() SourceInfoProvider
	GetContentType() string
	GetBody() []byte
	GetSize() int
	GetHeader(key string) interface{}
	GetHeaderByteSlice(key string) []byte
	GetHeaderString(key string) string
	GetHeaders() map[string]interface{}
	GetField(key string) interface{}
	GetFieldByteSlice(key string) []byte
	GetFieldString(key string) string
	GetFieldInt(key string) (int, error)
	GetFields() map[string]interface{}
	GetTimestamp() time.Time
	GetPath() string
	GetURL() string
	GetMethod() string
}
