package main

import (
	"time"

	"github.com/nuclio/nuclio/pkg/processor"

	nuclio "github.com/nuclio/nuclio-sdk"
)

var (
	testID             = nuclio.NewID()
	testSourceProvider = &TestSourceInfoProvider{}
	// Make sure all values here are strings
	testHeaders = map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}
	testTime = time.Now().UTC()
)

// nuclio.SourceInfoProvider interface
type TestSourceInfoProvider struct{}

func (ti *TestSourceInfoProvider) GetClass() string { return "test class" }
func (ti *TestSourceInfoProvider) GetKind() string  { return "test kind" }

type TestEvent struct {
	processor.AbstractEvent
}

// nuclio.Event interface
func (te *TestEvent) GetVersion() int {
	return 7
}

func (te *TestEvent) GetID() nuclio.ID {
	return testID
}

func (te *TestEvent) GetSource() nuclio.SourceInfoProvider {
	return testSourceProvider
}

func (te *TestEvent) GetContentType() string {
	return "text/html"
}

func (te *TestEvent) GetBody() []byte {
	return []byte("body of proof")
}

func (te *TestEvent) GetSize() int {
	return 14
}

func (te *TestEvent) GetHeader(key string) interface{} {
	return testHeaders[key]
}

func (te *TestEvent) GetHeaderByteSlice(key string) []byte {
	val := testHeaders[key]
	if val == nil {
		return nil
	}
	return val.([]byte)
}

func (te *TestEvent) GetHeaderString(key string) string {
	val := testHeaders[key]
	if val == nil {
		return ""
	}
	return val.(string)
}

func (te *TestEvent) GetHeaders() map[string]interface{} {
	return testHeaders
}

func (te *TestEvent) GetTimestamp() time.Time {
	return testTime
}

func (te *TestEvent) GetPath() string {
	return "/path/to/test"
}

func (te *TestEvent) GetURL() string {
	return "https://github.com/nuclio/nuclio"
}

func (te *TestEvent) GetMethod() string {
	return "POST"
}
