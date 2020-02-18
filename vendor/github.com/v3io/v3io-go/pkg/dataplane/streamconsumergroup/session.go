package streamconsumergroup

import (
	v3io "github.com/v3io/v3io-go/pkg/dataplane"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type session struct {
	logger logger.Logger
	member *member
	state  *SessionState
	claims []Claim
}

func newSession(member *member,
	sessionState *SessionState) (Session, error) {

	return &session{
		logger: member.logger.GetChild("session"),
		member: member,
		state:  sessionState,
	}, nil
}

func (s *session) start() error {
	s.logger.DebugWith("Starting session")

	// for each shard we need handle, create a StreamConsumerGroupClaim object and start it
	for _, shardID := range s.state.Shards {
		claim, err := newClaim(s.member, shardID)
		if err != nil {
			return errors.Wrapf(err, "Failed creating stream consumer group claim for shard: %d", shardID)
		}

		// add to claims
		s.claims = append(s.claims, claim)
	}

	// tell the consumer group handler to set up
	s.logger.DebugWith("Triggering given handler Setup")
	if err := s.member.handler.Setup(s); err != nil {
		return errors.Wrap(err, "Failed to set up session")
	}

	s.logger.DebugWith("Starting claim consumption")
	for _, claim := range s.claims {
		if err := claim.start(); err != nil {
			return errors.Wrap(err, "Failed starting stream consumer group claim")
		}
	}

	return nil
}

func (s *session) stop() error {
	s.logger.DebugWith("Stopping session, triggering given handler cleanup")

	// tell the consumer group handler to set up
	if err := s.member.handler.Cleanup(s); err != nil {
		return errors.Wrap(err, "Failed to cleanup")
	}

	s.logger.DebugWith("Stopping claims")

	for _, claim := range s.claims {
		err := claim.stop()
		if err != nil {
			return errors.Wrap(err, "Failed starting stream consumer group claim")
		}
	}

	return nil
}

func (s *session) GetClaims() []Claim {
	return s.claims
}

func (s *session) GetMemberID() string {
	return s.member.id
}

func (s *session) MarkRecord(record *v3io.StreamRecord) error {
	err := s.member.sequenceNumberHandler.markShardSequenceNumber(*record.ShardID, record.SequenceNumber)
	if err != nil {
		return errors.Wrap(err, "Failed marking record")
	}

	return nil
}
