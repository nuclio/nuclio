package streamconsumergroup

import (
	"time"

	"github.com/v3io/v3io-go/pkg/common"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

var ErrShardNotFound = errors.New("Shard not found")
var ErrShardSequenceNumberAttributeNotFound = errors.New("Shard sequenceNumber attribute")

type sequenceNumberHandler struct {
	logger                                     logger.Logger
	member                                     *member
	markedShardSequenceNumbers                 []uint64
	stopMarkedShardSequenceNumberCommitterChan chan struct{}
	lastCommittedShardSequenceNumbers          []uint64
}

func newSequenceNumberHandler(member *member) (*sequenceNumberHandler, error) {

	return &sequenceNumberHandler{
		logger:                     member.logger.GetChild("sequenceNumberHandler"),
		member:                     member,
		markedShardSequenceNumbers: make([]uint64, member.streamConsumerGroup.totalNumShards),
		stopMarkedShardSequenceNumberCommitterChan: make(chan struct{}, 1),
	}, nil
}

func (snh *sequenceNumberHandler) start() error {
	snh.logger.DebugWith("Starting sequenceNumber handler")

	// stopped on stop()
	go snh.markedShardSequenceNumbersCommitter(snh.member.streamConsumerGroup.config.SequenceNumber.CommitInterval,
		snh.stopMarkedShardSequenceNumberCommitterChan)

	return nil
}

func (snh *sequenceNumberHandler) stop() error {
	snh.logger.DebugWith("Stopping sequenceNumber handler")

	select {
	case snh.stopMarkedShardSequenceNumberCommitterChan <- struct{}{}:
	default:
	}

	return nil
}

func (snh *sequenceNumberHandler) markShardSequenceNumber(shardID int, sequenceNumber uint64) error {
	snh.markedShardSequenceNumbers[shardID] = sequenceNumber

	return nil
}

func (snh *sequenceNumberHandler) markedShardSequenceNumbersCommitter(interval time.Duration, stopChan chan struct{}) {
	for {
		select {
		case <-time.After(interval):
			if err := snh.commitMarkedShardSequenceNumbers(); err != nil {
				snh.logger.WarnWith("Failed committing marked shard sequenceNumbers", "err", errors.GetErrorStackString(err, 10))
				continue
			}
		case <-stopChan:
			snh.logger.Debug("Stopped committing marked shard sequenceNumbers")

			// do the last commit
			if err := snh.commitMarkedShardSequenceNumbers(); err != nil {
				snh.logger.WarnWith("Failed committing marked shard sequenceNumbers on stop", "err", errors.GetErrorStackString(err, 10))
			}
			return
		}
	}
}

func (snh *sequenceNumberHandler) commitMarkedShardSequenceNumbers() error {
	var markedShardSequenceNumbersCopy []uint64

	// create a copy of the marked shard sequenceNumbers
	markedShardSequenceNumbersCopy = append(markedShardSequenceNumbersCopy, snh.markedShardSequenceNumbers...)

	// if there was no chance since last, do nothing
	if common.Uint64SlicesEqual(snh.lastCommittedShardSequenceNumbers, markedShardSequenceNumbersCopy) {
		return nil
	}

	snh.logger.DebugWith("Committing marked shard sequenceNumbers", "markedShardSequenceNumbersCopy", markedShardSequenceNumbersCopy)

	var failedShardIDs []int
	for shardID, sequenceNumber := range markedShardSequenceNumbersCopy {

		// the sequenceNumber array holds a sequenceNumber for all partitions, indexed by their id to allow for
		// faster writes (using a rw lock) only the relevant shards ever get populated
		if sequenceNumber == 0 {
			continue
		}

		if err := snh.member.streamConsumerGroup.setShardSequenceNumberInPersistency(shardID, sequenceNumber); err != nil {
			snh.logger.WarnWith("Failed committing shard sequenceNumber", "shardID", shardID,
				"sequenceNumber", sequenceNumber,
				"err", errors.GetErrorStackString(err, 10))

			failedShardIDs = append(failedShardIDs, shardID)
		}
	}

	if len(failedShardIDs) > 0 {
		return errors.Errorf("Failed committing marked shard sequenceNumbers in shards: %v", failedShardIDs)
	}

	snh.lastCommittedShardSequenceNumbers = markedShardSequenceNumbersCopy

	return nil
}
