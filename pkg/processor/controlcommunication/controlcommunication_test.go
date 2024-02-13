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

package controlcommunication

import (
	"context"
	"testing"
	"time"

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
	suite.Require().Len(suite.broker.Consumers, len(getAllControlMessageKinds()))
	suite.Require().Len(suite.broker.Consumers[0].channels, 2)
	suite.Require().Equal(suite.broker.Consumers[0].channels[0], controlMessageChannel1)
	suite.Require().Equal(suite.broker.Consumers[0].channels[1], controlMessageChannel2)

	// close the first channel, then unsubscribe it
	close(controlMessageChannel1)
	err := suite.broker.Unsubscribe(StreamMessageAckKind, controlMessageChannel1)
	suite.Require().NoError(err)

	// make sure the channel is unsubscribed
	suite.Require().Len(suite.broker.Consumers, len(getAllControlMessageKinds()))
	suite.Require().Len(suite.broker.Consumers[0].channels, 1)
	suite.Require().Equal(suite.broker.Consumers[0].channels[0], controlMessageChannel2)
}

func (suite *ControlCommunicationTestSuite) TestSendMessage() {
	controlMessageDrain1 := make(chan *ControlMessage)
	controlMessageDrain2 := make(chan *ControlMessage)
	defer close(controlMessageDrain1)
	defer close(controlMessageDrain2)

	drainDoneConsumer, err := suite.broker.getConsumer(DrainDoneMessageKind)
	suite.Require().NoError(err)

	for _, controlMessageChan := range []chan *ControlMessage{controlMessageDrain1, controlMessageDrain2} {
		err = suite.broker.Subscribe(DrainDoneMessageKind, controlMessageChan)
		suite.Require().NoError(err)
	}

	defer func() {
		err := suite.broker.Unsubscribe(StreamMessageAckKind, controlMessageDrain1)
		suite.Require().NoError(err)
		err = suite.broker.Unsubscribe(StreamMessageAckKind, controlMessageDrain2)
		suite.Require().NoError(err)
	}()

	// send DrainDone message
	suite.Require().Len(drainDoneConsumer.channels, 2)
	err = suite.broker.SendToConsumers(&ControlMessage{Kind: DrainDoneMessageKind})
	suite.Require().NoError(err)
	suite.Require().Len(drainDoneConsumer.channels, 1)

	message := <-controlMessageDrain1
	suite.Require().Equal(message.Kind, DrainDoneMessageKind)

	err = suite.broker.SendToConsumers(&ControlMessage{Kind: DrainDoneMessageKind})
	suite.Require().NoError(err)
	message = <-controlMessageDrain2
	suite.Require().Equal(message.Kind, DrainDoneMessageKind)
	suite.Require().Len(drainDoneConsumer.channels, 0)

}

func (suite *ControlCommunicationTestSuite) TestBroadcastMessage() {
	controlMessageExplicitAck1 := make(chan *ControlMessage)
	controlMessageExplicitAck2 := make(chan *ControlMessage)
	defer close(controlMessageExplicitAck1)
	defer close(controlMessageExplicitAck2)

	explicitAckConsumer, err := suite.broker.getConsumer(StreamMessageAckKind)
	suite.Require().NoError(err)

	for _, controlMessageChan := range []chan *ControlMessage{controlMessageExplicitAck1, controlMessageExplicitAck2} {
		err = suite.broker.Subscribe(StreamMessageAckKind, controlMessageChan)
		suite.Require().NoError(err)
	}
	defer func() {
		err := suite.broker.Unsubscribe(StreamMessageAckKind, controlMessageExplicitAck1)
		suite.Require().NoError(err)
		err = suite.broker.Unsubscribe(StreamMessageAckKind, controlMessageExplicitAck2)
		suite.Require().NoError(err)
	}()

	suite.Require().Len(explicitAckConsumer.channels, 2)

	broadcastingCtx, broadcastingDone := context.WithCancel(suite.ctx)
	// broadcasting message (will be blocked until the message is read from all chans)
	go func() {
		_ = suite.broker.SendToConsumers(&ControlMessage{Kind: StreamMessageAckKind, // nolint: errcheck
			Attributes: map[string]interface{}{"test": "broadcast"}})
		broadcastingDone()
	}()

	for _, controlMessageChan := range []chan *ControlMessage{controlMessageExplicitAck1, controlMessageExplicitAck2} {
		message := <-controlMessageChan
		suite.Require().Equal(message.Kind, StreamMessageAckKind)
		suite.Require().Equal(message.Attributes["test"], "broadcast")
	}

	select {
	case <-broadcastingCtx.Done():
		suite.logger.DebugWith("ExplicitAck control message was successfully broadcast")
	case <-time.After(1 * time.Second):
		suite.Fail("ExplicitAck control message didn't unblock consumer after message was read")
	}
}

func TestControlCommunicationTestSuite(t *testing.T) {
	suite.Run(t, new(ControlCommunicationTestSuite))
}
