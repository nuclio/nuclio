package streamconsumergroup

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/v3io/v3io-go/pkg/common"
	"github.com/v3io/v3io-go/pkg/dataplane"
	"github.com/v3io/v3io-go/pkg/errors"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

var errShardNotFound = errors.New("Shard not found")
var errShardSequenceNumberAttributeNotFound = errors.New("Shard sequenceNumber attribute")

type sequenceNumberHandler struct {
	logger                                     logger.Logger
	streamConsumerGroup                        *streamConsumerGroup
	markedShardSequenceNumbers                 []uint64
	markedShardSequenceNumbersLock             sync.RWMutex
	stopMarkedShardSequenceNumberCommitterChan chan struct{}
	lastCommittedShardSequenceNumbers          []uint64
}

func newSequenceNumberHandler(streamConsumerGroup *streamConsumerGroup) (*sequenceNumberHandler, error) {

	return &sequenceNumberHandler{
		logger:                     streamConsumerGroup.logger.GetChild("sequenceNumberHandler"),
		streamConsumerGroup:        streamConsumerGroup,
		markedShardSequenceNumbers: make([]uint64, streamConsumerGroup.totalNumShards),
		stopMarkedShardSequenceNumberCommitterChan: make(chan struct{}, 1),
	}, nil
}

func (snh *sequenceNumberHandler) start() error {
	snh.logger.DebugWith("Starting sequenceNumber handler")

	// stopped on stop()
	go snh.markedShardSequenceNumbersCommitter(snh.streamConsumerGroup.config.SequenceNumber.CommitInterval,
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

	// lock semantics are reverse - it's OK to write in parallel since each write goes
	// to a different cell in the array, but once a read is happening we need to stop the world
	snh.markedShardSequenceNumbersLock.RLock()
	snh.markedShardSequenceNumbers[shardID] = sequenceNumber
	snh.markedShardSequenceNumbersLock.RUnlock()

	return nil
}

func (snh *sequenceNumberHandler) getShardLocationFromPersistency(shardID int) (string, error) {
	snh.logger.DebugWith("Getting shard sequenceNumber from persistency", "shardID", shardID)

	shardPath, err := snh.streamConsumerGroup.getShardPath(shardID)
	if err != nil {
		return "", errors.Wrapf(err, "Failed getting shard path: %v", shardID)
	}

	seekShardInput := v3io.SeekShardInput{
		Path: shardPath,
	}

	// get the shard sequenceNumber from the item
	shardSequenceNumber, err := snh.getShardSequenceNumberFromItemAttributes(shardPath)
	if err != nil {

		// if the error is that the attribute wasn't found, but the shard was found - seek the shard
		// according to the configuration
		if err != errShardSequenceNumberAttributeNotFound {
			return "", errors.Wrap(err, "Failed to get shard sequenceNumber from item attributes")
		}

		seekShardInput.Type = snh.streamConsumerGroup.config.Claim.RecordBatchFetch.InitialLocation
	} else {

		// use sequence number
		seekShardInput.Type = v3io.SeekShardInputTypeSequence
		seekShardInput.StartingSequenceNumber = shardSequenceNumber + 1
	}

	return snh.getShardLocationWithSeek(shardPath, &seekShardInput)
}

// returns the sequenceNumber, an error re: the shard itself and an error re: the attribute in the shard
func (snh *sequenceNumberHandler) getShardSequenceNumberFromItemAttributes(shardPath string) (uint64, error) {
	response, err := snh.streamConsumerGroup.container.GetItemSync(&v3io.GetItemInput{
		Path:           shardPath,
		AttributeNames: []string{snh.getShardCommittedSequenceNumberAttributeName()},
	})

	if err != nil {
		errWithStatusCode, errHasStatusCode := err.(v3ioerrors.ErrorWithStatusCode)
		if !errHasStatusCode {
			return 0, errors.Wrap(err, "Got error without status code")
		}

		if errWithStatusCode.StatusCode() != http.StatusNotFound {
			return 0, errors.Wrap(err, "Failed getting shard item")
		}

		// TODO: remove after errors.Is support added
		snh.logger.DebugWith("Could not find shard, probably doesn't exist yet", "path", shardPath)

		return 0, errShardNotFound
	}

	defer response.Release()

	getItemOutput := response.Output.(*v3io.GetItemOutput)

	// return the attribute name
	sequenceNumber, err := getItemOutput.Item.GetFieldUint64(snh.getShardCommittedSequenceNumberAttributeName())
	if err != nil && err == v3ioerrors.ErrNotFound {
		return 0, errShardSequenceNumberAttributeNotFound
	}

	// return the sequenceNumber we found
	return sequenceNumber, nil
}

func (snh *sequenceNumberHandler) getShardLocationWithSeek(shardPath string, seekShardInput *v3io.SeekShardInput) (string, error) {

	snh.logger.DebugWith("Seeking shard", "shardPath", shardPath, "seekShardInput", seekShardInput)

	response, err := snh.streamConsumerGroup.container.SeekShardSync(seekShardInput)
	if err != nil {
		return "", errors.Wrap(err, "Failed to seek shard")
	}
	defer response.Release()

	location := response.Output.(*v3io.SeekShardOutput).Location

	snh.logger.DebugWith("Seek shard succeeded", "shardPath", shardPath, "location", location)

	return location, nil
}

func (snh *sequenceNumberHandler) getShardCommittedSequenceNumberAttributeName() string {
	return fmt.Sprintf("__%s_committed_sequence_number", snh.streamConsumerGroup.name)
}

func (snh *sequenceNumberHandler) setShardSequenceNumberInPersistency(shardID int, sequenceNumber uint64) error {
	snh.logger.DebugWith("Setting shard sequenceNumber in persistency", "shardID", shardID, "sequenceNumber", sequenceNumber)
	shardPath, err := snh.streamConsumerGroup.getShardPath(shardID)
	if err != nil {
		return errors.Wrapf(err, "Failed getting shard path: %v", shardID)
	}

	return snh.streamConsumerGroup.container.UpdateItemSync(&v3io.UpdateItemInput{
		Path: shardPath,
		Attributes: map[string]interface{}{
			snh.getShardCommittedSequenceNumberAttributeName(): sequenceNumber,
		},
	})
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
	snh.markedShardSequenceNumbersLock.Lock()
	markedShardSequenceNumbersCopy = append(markedShardSequenceNumbersCopy, snh.markedShardSequenceNumbers...)
	snh.markedShardSequenceNumbersLock.Unlock()

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

		if err := snh.setShardSequenceNumberInPersistency(shardID, sequenceNumber); err != nil {
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
