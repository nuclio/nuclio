//go:build test_unit

/*
Copyright 2017 The Nuclio Authors.

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

package rpc

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/zap"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/suite"
)

var (
	testID                  = nuclio.ID(uuid.NewV4().String())
	testTriggerInfoProvider = &TestTriggerInfoProvider{}

	testHeaders = map[string]interface{}{
		"h1": "hv1",
		"h2": 2,
	}
	testFields = map[string]interface{}{
		"f1": "fv1",
		"f2": 0xF2,
	}
	testTime = time.Now().UTC()
)

// nuclio.TriggerInfoProvider interface
type TestTriggerInfoProvider struct{}

func (ti *TestTriggerInfoProvider) GetClass() string { return "test class" }
func (ti *TestTriggerInfoProvider) GetKind() string  { return "test kind" }
func (ti *TestTriggerInfoProvider) GetName() string  { return "test name" }

type TestEvent struct {
	// We don't embed nuclio.AbstractEvent so we'll have all methods
}

func (te *TestEvent) GetID() nuclio.ID {
	return testID
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
	return testHeaders
}

func (te *TestEvent) GetFields() map[string]interface{} {
	return testFields
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

func (te *TestEvent) GetTriggerInfo() nuclio.TriggerInfoProvider {
	return testTriggerInfoProvider
}

func (te *TestEvent) GetHeader(key string) interface{} {
	return testHeaders[key]
}
func (te *TestEvent) GetHeaderByteSlice(key string) []byte {
	return testHeaders[key].([]byte)
}
func (te *TestEvent) GetHeaderString(key string) string {
	return testHeaders[key].(string)
}
func (te *TestEvent) GetHeaderInt(key string) (int, error) {
	return testHeaders[key].(int), nil
}

func (te *TestEvent) GetField(key string) interface{} {
	return testFields[key]
}
func (te *TestEvent) GetFieldByteSlice(key string) []byte {
	return testFields[key].([]byte)
}
func (te *TestEvent) GetFieldString(key string) string {
	return testFields[key].(string)
}
func (te *TestEvent) GetFieldInt(key string) (int, error) {
	return testFields[key].(int), nil
}
func (te *TestEvent) SetID(id nuclio.ID)                                    {}
func (te *TestEvent) SetTriggerInfoProvider(tip nuclio.TriggerInfoProvider) {}

type EventJSONEncoderSuite struct {
	suite.Suite
}

func (suite *EventJSONEncoderSuite) TestEncode() {
	require := suite.Require()
	logger, err := nucliozap.NewNuclioZapTest("test")
	require.NoError(err, "Can't create logger")

	var buf bytes.Buffer
	enc := NewEventJSONEncoder(logger, &buf)
	testEvent := &TestEvent{}
	err = enc.Encode(testEvent)
	require.NoError(err, "Can't encode event")

	// Make sure we got a valid JSON object
	out := make(map[string]interface{})
	dec := json.NewDecoder(&buf)
	err = dec.Decode(&out)
	require.NoError(err, "Can't decode event")

	require.Equal(testID, nuclio.ID(out["id"].(string)), "bad id")
	require.Equal(testEvent.GetContentType(), out["content-type"], "bad content type")

	headers, ok := out["headers"].(map[string]interface{})
	require.True(ok, "bad headers type")
	require.Equal(headers["h1"], testHeaders["h1"], "bad h1 header")
	// Go converts all numbers to floats
	require.Equal(int(headers["h2"].(float64)), testHeaders["h2"], "bad h2 header")

	fields, ok := out["fields"].(map[string]interface{})
	require.True(ok, "bad fields type")
	require.Equal(fields["f1"], testFields["f1"], "bad f1 field")
	// Go converts all numbers to floats
	require.Equal(int(fields["f2"].(float64)), testFields["f2"], "bad f2 field")

	triggerInfo := out["trigger"].(map[string]interface{})
	require.Equal(testTriggerInfoProvider.GetKind(), triggerInfo["kind"], "bad trigger kind")
	require.Equal(testTriggerInfoProvider.GetName(), triggerInfo["name"], "bad trigger name")

	require.Equal(testEvent.GetMethod(), out["method"], "bad method")
	require.Equal(testEvent.GetPath(), out["path"], "bad path")
	require.Equal(testEvent.GetURL(), out["url"], "bad URL")

	shardID := float64(testEvent.GetShardID())
	require.Equal(shardID, out["shard_id"], "bad shard ID")

	numShards := float64(testEvent.GetTotalNumShards())
	require.Equal(numShards, out["num_shards"], "bad number of shards")

	timeStamp := float64(testEvent.GetTimestamp().UTC().Unix())
	require.Equal(timeStamp, out["timestamp"], "bad timestamp")

	require.Equal(testEvent.GetType(), out["type"], "bad type")
	require.Equal(testEvent.GetTypeVersion(), out["type_version"], "bad type version")
	require.Equal(testEvent.GetVersion(), out["version"], "bad version")
}

func TestEventJSONEncoder(t *testing.T) {
	suite.Run(t, new(EventJSONEncoderSuite))
}
