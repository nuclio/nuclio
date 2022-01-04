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

package cloudevent

import (
	"fmt"
	"testing"
	"time"

	"github.com/nuclio/nuclio-sdk-go"
	"github.com/stretchr/testify/suite"
)

type structuredTestSuite struct {
	suite.Suite
	structuredEvent Structured
	mockEvent       mockEvent
}

func (suite *structuredTestSuite) SetupTest() {
	suite.mockEvent = mockEvent{}
}

func (suite *structuredTestSuite) TestSuccess() {
	suite.mockEvent.On("SetTriggerInfoProvider", &suite.structuredEvent)

	now := time.Now().UTC().Format(time.RFC3339)

	// format the structured body
	structuredBody := fmt.Sprintf(`{
	"eventType": "testEventType",
	"eventTypeVersion": "testEventTypeVersion",
	"cloudEventsVersion": "testCloudEventsVersion",
	"source": "testSource",
	"eventID": "testEventID",
	"eventTime": "%s",
	"contentType": "testContentType",
	"extensions": {"testExtensionKey1": "testExtensionValue1", "testExtensionKey2": 2},
	"data": "testData"
}`, now)

	suite.mockEvent.On("GetBody").Return([]byte(structuredBody))

	// set the event - will parse the body
	err := suite.structuredEvent.SetEvent(&suite.mockEvent)
	suite.Require().NoError(err)

	// verify fields coming from the cloud event
	suite.Require().Equal("testEventType", suite.structuredEvent.GetType())
	suite.Require().Equal("testEventTypeVersion", suite.structuredEvent.GetTypeVersion())
	suite.Require().Equal("testCloudEventsVersion", suite.structuredEvent.GetVersion())
	suite.Require().Equal("testSource", suite.structuredEvent.GetTriggerInfo().GetKind())
	suite.Require().Equal(nuclio.ID("testEventID"), suite.structuredEvent.GetID())
	suite.Require().Equal(now, suite.structuredEvent.GetTimestamp().Format(time.RFC3339))
	suite.Require().Equal("testContentType", suite.structuredEvent.GetContentType())
	suite.Require().Equal([]byte("testData"), suite.structuredEvent.GetBody())

	// verify expectations
	suite.mockEvent.AssertExpectations(suite.T())
}

func (suite *structuredTestSuite) TestSuccessWithObjectData() {
	suite.mockEvent.On("SetTriggerInfoProvider", &suite.structuredEvent)

	now := time.Now().UTC().Format(time.RFC3339)

	// format the structured body
	structuredBody := fmt.Sprintf(`{
	"eventType": "testEventType",
	"eventTypeVersion": "testEventTypeVersion",
	"cloudEventsVersion": "testCloudEventsVersion",
	"source": "testSource",
	"eventID": "testEventID",
	"eventTime": "%s",
	"contentType": "testContentType",
	"data": {"a": "b", "c": 1}
}`, now)

	suite.mockEvent.On("GetBody").Return([]byte(structuredBody))

	// set the event - will parse the body
	err := suite.structuredEvent.SetEvent(&suite.mockEvent)
	suite.Require().NoError(err)

	// verify fields coming from the cloud event
	suite.Require().Equal("testEventType", suite.structuredEvent.GetType())
	suite.Require().Equal("testEventTypeVersion", suite.structuredEvent.GetTypeVersion())
	suite.Require().Equal("testCloudEventsVersion", suite.structuredEvent.GetVersion())
	suite.Require().Equal("testSource", suite.structuredEvent.GetTriggerInfo().GetKind())
	suite.Require().Equal(nuclio.ID("testEventID"), suite.structuredEvent.GetID())
	suite.Require().Equal(now, suite.structuredEvent.GetTimestamp().Format(time.RFC3339))
	suite.Require().Equal("testContentType", suite.structuredEvent.GetContentType())
	suite.Require().Equal(map[string]interface{}{
		"a": "b",
		"c": float64(1),
	}, suite.structuredEvent.GetBodyObject())

	// verify expectations
	suite.mockEvent.AssertExpectations(suite.T())
}

func TestStructuredTestSuite(t *testing.T) {
	suite.Run(t, new(structuredTestSuite))
}
