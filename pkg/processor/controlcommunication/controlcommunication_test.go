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

package controlcommunication

import (
	"context"
	"testing"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type ControlCommunicationTestSuite struct {
	suite.Suite
	logger logger.Logger
	ctx    context.Context
	broker *AbstractControlMessageBroker
}

func (suite *ControlCommunicationTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.ctx = context.Background()
	suite.broker = NewAbstractControlMessageBroker()
}

func (suite *ControlCommunicationTestSuite) TestSubscribeUnsubscribe() {

	// create 2 control message channels
	controlMessageChannel1 := make(chan *ControlMessage)
	controlMessageChannel2 := make(chan *ControlMessage)

	// subscribe the channels to an explicit ack control message kind
	for _, controlMessageChannel := range []chan *ControlMessage{controlMessageChannel1, controlMessageChannel2} {
		err := suite.broker.Subscribe(StreamMessageAckKind, controlMessageChannel)
		suite.Require().NoError(err)
	}

	// make sure the channel is subscribed
	suite.Require().Len(suite.broker.Consumers, 1)
	suite.Require().Len(suite.broker.Consumers[0].Channels, 2)
	suite.Require().Equal(suite.broker.Consumers[0].Channels[0], controlMessageChannel1)
	suite.Require().Equal(suite.broker.Consumers[0].Channels[1], controlMessageChannel2)

	// close the first channel, then unsubscribe it
	close(controlMessageChannel1)
	err := suite.broker.Unsubscribe(StreamMessageAckKind, controlMessageChannel1)
	suite.Require().NoError(err)

	// make sure the channel is unsubscribed
	suite.Require().Len(suite.broker.Consumers, 1)
	suite.Require().Len(suite.broker.Consumers[0].Channels, 1)
	suite.Require().Equal(suite.broker.Consumers[0].Channels[0], controlMessageChannel2)
}

func TestControlCommunicationTestSuite(t *testing.T) {
	suite.Run(t, new(ControlCommunicationTestSuite))
}
