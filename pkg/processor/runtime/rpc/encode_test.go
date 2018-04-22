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
	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

var (
	testID                  = nuclio.ID(xid.New().String())
	testTriggerInfoProvider = &TestTriggerInfoProvider{}
	// Make sure all values here are strings
	testHeaders = map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}
	testTime = time.Now().UTC()
)

// nuclio.TriggerInfoProvider interface
type TestTriggerInfoProvider struct{}

func (ti *TestTriggerInfoProvider) GetClass() string { return "test class" }
func (ti *TestTriggerInfoProvider) GetKind() string  { return "test kind" }

type TestEvent struct {
	nuclio.AbstractEvent
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

type EventJSONEncoderSuite struct {
	suite.Suite
}

func (suite *EventJSONEncoderSuite) TestEncode() {
	logger, err := nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err, "Can't create logger")

	var buf bytes.Buffer
	enc := NewEventJSONEncoder(logger, &buf)
	testEvent := &TestEvent{}
	testEvent.SetTriggerInfoProvider(testTriggerInfoProvider)
	err = enc.Encode(testEvent)
	suite.Require().NoError(err, "Can't encode event")

	// Make sure we got a valid JSON object
	out := make(map[string]interface{})
	dec := json.NewDecoder(&buf)
	err = dec.Decode(&out)
	suite.Require().NoError(err, "Can't decode event")

	// Check a value (TODO: Check all fields)
	suite.Require().Equal(testEvent.GetID(), testID)
}

func TestEventJSONEncoder(t *testing.T) {
	suite.Run(t, new(EventJSONEncoderSuite))
}
