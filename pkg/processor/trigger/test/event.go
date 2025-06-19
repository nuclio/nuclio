/*
Copyright 2025 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package triggertest

import (
	"time"

	"github.com/nuclio/nuclio-sdk-go"
)

var (
	TestHeaders = map[string]interface{}{
		"h1": "hv1",
		"h2": 2,
	}
	TestFields = map[string]interface{}{
		"f1": "fv1",
		"f2": 0xF2,
	}
)

type TestTriggerInfoProvider struct{}

func (ti *TestTriggerInfoProvider) GetClass() string { return "test class" }
func (ti *TestTriggerInfoProvider) GetKind() string  { return "test kind" }
func (ti *TestTriggerInfoProvider) GetName() string  { return "test name" }

type TestEvent struct {
	testID                  nuclio.ID
	testTriggerInfoProvider nuclio.TriggerInfoProvider
	// We don't embed nuclio.AbstractEvent so we'll have all methods
}

func NewTestEvent(id nuclio.ID, triggerInfoProvide nuclio.TriggerInfoProvider) *TestEvent {
	return &TestEvent{
		testID:                  id,
		testTriggerInfoProvider: triggerInfoProvide,
	}
}

func (te *TestEvent) GetID() nuclio.ID {
	return te.testID
}

func (te *TestEvent) GetContentType() string {
	return "text/html"
}

func (te *TestEvent) GetBody() []byte {
	return []byte("body of proof")
}

func (te *TestEvent) GetBodyObject() interface{} {
	return te.GetBody
}

func (te *TestEvent) GetHeaders() map[string]interface{} {
	return TestHeaders
}

func (te *TestEvent) GetFields() map[string]interface{} {
	return TestFields
}

func (te *TestEvent) GetTimestamp() time.Time {
	return time.Now().UTC()
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

func (te *TestEvent) GetShardID() int {
	return 9
}

func (te *TestEvent) GetTotalNumShards() int {
	return 32
}

func (te *TestEvent) GetType() string {
	return "test event type"
}

func (te *TestEvent) GetTypeVersion() string {
	return "test event type version"
}

func (te *TestEvent) GetVersion() string {
	return "test event version"
}

// GetLastInBatch returns whether the event is the last event in a trigger specific batch
func (te *TestEvent) GetLastInBatch() bool {
	return false
}

// GetOffset returns the offset of the event
func (te *TestEvent) GetOffset() int {
	return 0
}

func (te *TestEvent) GetTopic() string {
	return ""
}

func (te *TestEvent) GetTriggerInfo() nuclio.TriggerInfoProvider {
	return te.testTriggerInfoProvider
}

func (te *TestEvent) GetHeader(key string) interface{} {
	return TestHeaders[key]
}
func (te *TestEvent) GetHeaderByteSlice(key string) []byte {
	return TestHeaders[key].([]byte)
}
func (te *TestEvent) GetHeaderString(key string) string {
	return TestHeaders[key].(string)
}
func (te *TestEvent) GetHeaderInt(key string) (int, error) {
	return TestHeaders[key].(int), nil
}

func (te *TestEvent) GetField(key string) interface{} {
	return TestFields[key]
}
func (te *TestEvent) GetFieldByteSlice(key string) []byte {
	return TestFields[key].([]byte)
}
func (te *TestEvent) GetFieldString(key string) string {
	return TestFields[key].(string)
}
func (te *TestEvent) GetFieldInt(key string) (int, error) {
	return TestFields[key].(int), nil
}
func (te *TestEvent) SetID(id nuclio.ID) {}

func (te *TestEvent) SetTriggerInfoProvider(tip nuclio.TriggerInfoProvider) {}
