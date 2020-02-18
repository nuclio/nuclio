package streamconsumergroup

import (
	"fmt"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/rs/xid"
)

type member struct {
	id                    string
	logger                logger.Logger
	streamConsumerGroup   *streamConsumerGroup
	stateHandler          *stateHandler
	sequenceNumberHandler *sequenceNumberHandler
	handler               Handler
	session               Session
}

func NewMember(streamConsumerGroupInterface StreamConsumerGroup, name string) (Member, error) {
	var err error

	streamConsumerGroupInstance, ok := streamConsumerGroupInterface.(*streamConsumerGroup)
	if !ok {
		return nil, errors.Errorf("Expected streamConsumerGroupInterface of type streamConsumerGroup, got %T", streamConsumerGroupInterface)
	}

	// add uniqueness
	id := fmt.Sprintf("%s-%s", name, xid.New().String())

	newMember := member{
		logger:              streamConsumerGroupInstance.logger.GetChild(id),
		id:                  id,
		streamConsumerGroup: streamConsumerGroupInstance,
	}

	// create & start a state handler for the stream
	newMember.stateHandler, err = newStateHandler(&newMember)
	if err != nil {
		return nil, errors.Wrap(err, "Failed creating stream consumer group state handler")
	}

	err = newMember.stateHandler.start()
	if err != nil {
		return nil, errors.Wrap(err, "Failed starting stream consumer group state handler")
	}

	// create & start an location handler for the stream
	newMember.sequenceNumberHandler, err = newSequenceNumberHandler(&newMember)
	if err != nil {
		return nil, errors.Wrap(err, "Failed creating stream consumer group location handler")
	}

	// if there's no member name, just observe
	err = newMember.sequenceNumberHandler.start()
	if err != nil {
		return nil, errors.Wrap(err, "Failed starting stream consumer group state handler")
	}

	return &newMember, nil
}

func (m *member) Consume(handler Handler) error {
	m.logger.DebugWith("Starting consumption of consumer group")

	m.handler = handler

	// get the state (holding our shards)
	sessionState, err := m.stateHandler.getOrCreateSessionState(m.id)
	if err != nil {
		return errors.Wrap(err, "Failed getting stream consumer group member state")
	}

	// create a session object from our state
	m.session, err = newSession(m, sessionState)
	if err != nil {
		return errors.Wrap(err, "Failed creating stream consumer group session")
	}

	// start it
	return m.session.start()
}

func (m *member) Close() error {
	m.logger.DebugWith("Closing consumer group")

	if err := m.stateHandler.stop(); err != nil {
		return errors.Wrapf(err, "Failed stopping state handler")
	}
	if err := m.sequenceNumberHandler.stop(); err != nil {
		return errors.Wrapf(err, "Failed stopping location handler")
	}

	if m.session != nil {
		if err := m.session.stop(); err != nil {
			return errors.Wrap(err, "Failed stopping member session")
		}
	}

	return nil
}
