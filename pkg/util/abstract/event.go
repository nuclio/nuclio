package abstract

import (
	"errors"
	"time"

	"github.com/nuclio/nuclio-sdk"
)

var ErrUnsupported = errors.New("Event does not support this interface")

// Abstract implementation of event

type AbstractEvent struct {
	sourceInfoProvider nuclio.SourceInfoProvider
	id                 nuclio.ID
	emptyByteArray     []byte
	emptyHeaders       map[string]interface{}
	emptyTime          time.Time
}

func (ae *AbstractEvent) GetVersion() int {
	return 0
}

func (ae *AbstractEvent) SetSourceProvider(sourceInfoProvider nuclio.SourceInfoProvider) {
	ae.sourceInfoProvider = sourceInfoProvider
}

func (ae *AbstractEvent) GetSource() nuclio.SourceInfoProvider {
	return ae.sourceInfoProvider
}

func (ae *AbstractEvent) GetID() nuclio.ID {
	return ae.id
}

func (ae *AbstractEvent) SetID(id nuclio.ID) {
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

func (ae *AbstractEvent) GetHeader(key string) interface{} {
	return nil
}

func (ae *AbstractEvent) GetHeaderByteSlice(key string) []byte {
	return ae.emptyByteArray
}

func (ae *AbstractEvent) GetHeaderString(key string) string {
	return string(ae.GetHeaderByteSlice(key))
}

func (ae *AbstractEvent) GetHeaders() map[string]interface{} {
	return ae.emptyHeaders
}

func (ae *AbstractEvent) GetTimestamp() time.Time {
	return ae.emptyTime
}

func (ae *AbstractEvent) GetPath() string {
	return ""
}

func (ae *AbstractEvent) GetURL() string {
	return ""
}

func (ae *AbstractEvent) GetMethod() string {
	return ""
}

func (ae *AbstractEvent) GetField(key string) interface{} {
	return nil
}

func (ae *AbstractEvent) GetFieldByteSlice(key string) []byte {
	return nil
}

func (ae *AbstractEvent) GetFieldString(key string) string {
	return ""
}

func (ae *AbstractEvent) GetFieldInt(key string) (int, error) {
	return 0, ErrUnsupported
}

func (ae *AbstractEvent) GetFields() map[string]interface{} {
	return nil
}
