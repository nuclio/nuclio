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
	"testing"
	"time"

	"github.com/nuclio/nuclio-sdk-go"
	"github.com/stretchr/testify/suite"
)

type binaryTestSuite struct {
	suite.Suite
	binaryEvent Binary
	mockEvent   mockEvent
}

func (suite *binaryTestSuite) TestSuccess() {
	suite.mockEvent.On("SetTriggerInfoProvider", &suite.binaryEvent)

	now := time.Now().UTC().Format(time.RFC3339)
	body := []byte("testBody")
	contentType := "testContentType"

	suite.mockEvent.On("GetBody").Return(body)
	suite.mockEvent.On("GetHeaderString", "CE-EventType").Return("testEventType")
	suite.mockEvent.On("GetHeaderString", "CE-EventID").Return("testEventID")
	suite.mockEvent.On("GetHeaderString", "CE-EventTypeVersion").Return("testEventTypeVersion")
	suite.mockEvent.On("GetHeaderString", "CE-CloudEventsVersion").Return("testCloudEventsVersion")
	suite.mockEvent.On("GetHeaderString", "CE-Source").Return("testSource")
	suite.mockEvent.On("GetHeaderString", "CE-Source").Return("testSource")
	suite.mockEvent.On("GetHeaderString", "CE-EventTime").Return(now)
	suite.mockEvent.On("GetContentType").Return(contentType)

	// set the event
	err := suite.binaryEvent.SetEvent(&suite.mockEvent)
	suite.Require().NoError(err)

	// verify fields coming from the cloud event
	suite.Require().Equal("testEventType", suite.binaryEvent.GetType())
	suite.Require().Equal("testEventTypeVersion", suite.binaryEvent.GetTypeVersion())
	suite.Require().Equal("testCloudEventsVersion", suite.binaryEvent.GetVersion())
	suite.Require().Equal("testSource", suite.binaryEvent.GetTriggerInfo().GetKind())
	suite.Require().Equal(nuclio.ID("testEventID"), suite.binaryEvent.GetID())
	suite.Require().Equal(now, suite.binaryEvent.GetTimestamp().Format(time.RFC3339))
	suite.Require().Equal("testContentType", suite.binaryEvent.GetContentType())
	suite.Require().Equal(body, suite.binaryEvent.GetBody())

	// verify expectations
	suite.mockEvent.AssertExpectations(suite.T())
}

func TestBinaryTestSuite(t *testing.T) {
	suite.Run(t, new(binaryTestSuite))
}
