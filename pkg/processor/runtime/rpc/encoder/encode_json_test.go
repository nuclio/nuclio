//go:build test_unit

/*
Copyright 2023 The Nuclio Authors.

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

package encoder

import (
	"bytes"
	"encoding/json"
	"testing"

	triggertest "github.com/nuclio/nuclio/pkg/processor/trigger/test"

	"github.com/google/uuid"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

var (
	testID                  = nuclio.ID(uuid.New().String())
	testTriggerInfoProvider = &triggertest.TestTriggerInfoProvider{}
)

type EventJSONEncoderSuite struct {
	suite.Suite
}

func (suite *EventJSONEncoderSuite) TestEncode() {
	require := suite.Require()
	logger, err := nucliozap.NewNuclioZapTest("test")
	require.NoError(err, "Can't create logger")

	var buf bytes.Buffer
	enc := NewEventJSONEncoder(logger, &buf)
	testEvent := triggertest.TestEvent{}
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
	require.Equal(headers["h1"], triggertest.TestHeaders["h1"], "bad h1 header")
	// Go converts all numbers to floats
	require.Equal(int(headers["h2"].(float64)), triggertest.TestHeaders["h2"], "bad h2 header")

	fields, ok := out["fields"].(map[string]interface{})
	require.True(ok, "bad fields type")
	require.Equal(fields["f1"], triggertest.TestHeaders["f1"], "bad f1 field")
	// Go converts all numbers to floats
	require.Equal(int(fields["f2"].(float64)), triggertest.TestHeaders["f2"], "bad f2 field")

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
