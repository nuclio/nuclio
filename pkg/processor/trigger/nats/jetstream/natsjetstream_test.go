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

package natsjetstream

import (
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/logger"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	trigger natsjetstream
	logger  logger.Logger
}

func (suite *TestSuite) TestStreamAndConsumerConfiguration() {
	for _, testCase := range []struct {
		name                   string
		stream                 string
		consumer               string
		expectedFailure        bool
	}{
		{
			name:                   "Stream and Consumer specified",
			stream:                 "mystream",
			consumer:               "myconsumer",
			expectedFailure:        false,
		},
		{
			name:                   "Stream not specified",
			stream:                 "",
			consumer:               "myconsumer",
			expectedFailure:        true,
		},
		{
			name:            	"Consumer not specified",
			stream:                 "mystream",
			consumer:               "",
			expectedFailure: 	true,
		},
	} {
		triggerInstance := &functionconfig.Trigger{
			Attributes: map[string]interface{}{
				"stream":   testCase.stream,
				"consumer": testCase.consumer,
			},
		}
		suite.Run(testCase.name, func() {
			configuration, err := NewConfiguration(testCase.name,
				triggerInstance,
				&runtime.Configuration{
					Configuration: &processor.Configuration{
						Config: functionconfig.Config{},
					},
				},
				suite.logger)
			if testCase.expectedFailure {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(testCase.stream, configuration.Stream, "Bad stream value")
				suite.Require().Equal(testCase.consumer, configuration.Consumer, "Bad consumer value")
			}
		})
	}
}
func (suite *TestSuite) TestReconnectConfiguration() {
	for _, testCase := range []struct {
		name                   string
		allowReconnect         bool
		maxReconnect           int
		expectedMaxReconnect   int
	}{
		{
			name:                   "MaxReconnect specified",
			allowReconnect:         true,
			maxReconnect:           2,
			expectedMaxReconnect:   2,
		},
		{
			name:                   "MaxReconnect not specified",
			allowReconnect:         true,
			maxReconnect:           0,
			expectedMaxReconnect:   60,
		},
		{
			name:                   "MaxReconnect negative",
			allowReconnect:         true,
			maxReconnect:           -1,
			expectedMaxReconnect:   -1,
		},
		{
			name:                   "No Reconnect",
			allowReconnect:         false,
			maxReconnect:           -1,
			expectedMaxReconnect:   -1,
		},
	} {
		triggerInstance := &functionconfig.Trigger{
			Attributes: map[string]interface{}{
				"stream":   "mystream",
				"consumer": "myconsumer",
				"allowReconnect": testCase.allowReconnect,
				"maxReconnect": testCase.maxReconnect,
			},
		}
		suite.Run(testCase.name, func() {
			configuration, err := NewConfiguration(testCase.name,
				triggerInstance,
				&runtime.Configuration{
					Configuration: &processor.Configuration{
						Config: functionconfig.Config{},
					},
				},
				suite.logger)
			suite.Require().NoError(err)
			suite.Require().Equal(testCase.expectedMaxReconnect, configuration.MaxReconnect, "Bad maxReconnect value")
			suite.Require().Equal(testCase.allowReconnect, configuration.AllowReconnect, "Bad allowReconnect value")
		})
	}
}

func (suite *TestSuite) TestReconnectWaitConfiguration() {
	for _, testCase := range []struct {
		name                   string
		reconnectWait          string
		reconnectJitter        string
		expectedReconnectWait  time.Duration
		expectedReconnectJitter time.Duration
		expectedFailure 	bool
	}{
		{
			name:                   "Time specified",
			reconnectWait:          "2s",
			reconnectJitter:        "3s",
			expectedReconnectWait:  2 * time.Second,
			expectedReconnectJitter:  3 * time.Second,
			expectedFailure: false,
		},
		{
			name:                   "Time not specified",
			reconnectWait:          "",
			reconnectJitter:        "",
			expectedReconnectWait:  2 * time.Second,
			expectedReconnectJitter:  100 * time.Millisecond,
			expectedFailure: false,
		},
		{
			name:            "Wrong wait value",
			reconnectWait:          "wait",
			expectedFailure: true,
		},
		{
			name:            "Wrong jitter value",
			reconnectJitter:        "jitter",
			expectedFailure: true,
		},
	} {
		triggerInstance := &functionconfig.Trigger{
			Attributes: map[string]interface{}{
				"stream":   "mystream",
				"consumer": "myconsumer",
				"reconnectWait": testCase.reconnectWait,
				"reconnectJitter": testCase.reconnectJitter,
			},
		}
		suite.Run(testCase.name, func() {
			configuration, err := NewConfiguration(testCase.name,
				triggerInstance,
				&runtime.Configuration{
					Configuration: &processor.Configuration{
						Config: functionconfig.Config{},
					},
				},
				suite.logger)
			if testCase.expectedFailure {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(testCase.expectedReconnectWait, configuration.reconnectWait, "Bad reconnectWait value")
				suite.Require().Equal(testCase.expectedReconnectJitter, configuration.reconnectJitter, "Bad reconnectJitter value")
			}
		})
	}
}

func (suite *TestSuite) TestTimeoutConfiguration() {
	for _, testCase := range []struct {
		name                   string
		timeout                string
		drainTimeout           string
		flusherTimeout         string
		expectedTimeout        time.Duration
		expectedDrainTimeout   time.Duration
		expectedFlusherTimeout time.Duration
		expectedFailure        bool
	}{
		{
			name:                   "Time specified",
			timeout:                "2s",
			drainTimeout:           "3s",
			flusherTimeout:         "4s",
			expectedTimeout:        2 * time.Second,
			expectedDrainTimeout:   3 * time.Second,
			expectedFlusherTimeout: 4 * time.Second,
			expectedFailure: false,
		},
		{
			name:                   "Time not specified",
			timeout:                "",
			drainTimeout:           "",
			flusherTimeout:         "",
			expectedTimeout:        2 * time.Second,
			expectedDrainTimeout:   30 * time.Second,
			expectedFlusherTimeout: 1 * time.Minute,
			expectedFailure: false,
		},
		{
			name:            "Wrong timeout value",
			timeout:         "timeout",
			expectedFailure: true,
		},
		{
			name:            "Wrong drainTimeout value",
			drainTimeout:    "timeout",
			expectedFailure: true,
		},
		{
			name:            "Wrong flusherTimeout value",
			flusherTimeout:  "timeout",
			expectedFailure: true,
		},
	} {
		triggerInstance := &functionconfig.Trigger{
			Attributes: map[string]interface{}{
				"stream":   "mystream",
				"consumer": "myconsumer",
				"timeout": testCase.timeout,
				"drainTimeout": testCase.drainTimeout,
				"flusherTimeout": testCase.flusherTimeout,
			},
		}
		suite.Run(testCase.name, func() {
			configuration, err := NewConfiguration(testCase.name,
				triggerInstance,
				&runtime.Configuration{
					Configuration: &processor.Configuration{
						Config: functionconfig.Config{},
					},
				},
				suite.logger)
			if testCase.expectedFailure {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(testCase.expectedTimeout, configuration.timeout, "Bad timeout value")
				suite.Require().Equal(testCase.expectedDrainTimeout, configuration.drainTimeout, "Bad drainTimeout value")
				suite.Require().Equal(testCase.expectedFlusherTimeout, configuration.flusherTimeout, "Bad flusherTimeout value")
			}
		})
	}
}

func (suite *TestSuite) TestPingConfiguration() {
	for _, testCase := range []struct {
		name                   string
		pingInterval           string
		maxPingsOut            int
		expectedPingInterval   time.Duration
		expectedMaxPingsOut    int
		expectedFailure        bool
	}{
		{
			name:                   "Ping specified",
			pingInterval:           "2s",
			maxPingsOut:            5,
			expectedPingInterval:   2 * time.Second,
			expectedMaxPingsOut:    5,
			expectedFailure: false,
		},
		{
			name:                   "PingInterval not specified",
			pingInterval:           "",
			maxPingsOut:            5,
			expectedPingInterval:   2 * time.Minute,
			expectedMaxPingsOut:    5,
			expectedFailure: false,
		},
		{
			name:                   "MaxPingsOut not specified",
			pingInterval:           "2s",
			maxPingsOut:            0,
			expectedPingInterval:   2 * time.Second,
			expectedMaxPingsOut:    2,
			expectedFailure: false,
		},
		{
			name:            "Wrong pingInterval value",
			pingInterval:          "pingInterval",
			expectedFailure: true,
		},
	} {
		triggerInstance := &functionconfig.Trigger{
			Attributes: map[string]interface{}{
				"stream":   "mystream",
				"consumer": "myconsumer",
				"pingInterval": testCase.pingInterval,
				"maxPingsOut": testCase.maxPingsOut,
			},
		}
		suite.Run(testCase.name, func() {
			configuration, err := NewConfiguration(testCase.name,
				triggerInstance,
				&runtime.Configuration{
					Configuration: &processor.Configuration{
						Config: functionconfig.Config{},
					},
				},
				suite.logger)
			if testCase.expectedFailure {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(testCase.expectedPingInterval, configuration.pingInterval, "Bad pingInterval value")
				suite.Require().Equal(testCase.expectedMaxPingsOut, configuration.MaxPingsOut, "Bad maxPingsOut value")
			}
		})
	}
}

func TestNatsJetstreamSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
