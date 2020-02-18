package streamconsumergroup

import (
	"fmt"
	"path"
	"strconv"
	"time"

	"github.com/v3io/v3io-go/pkg/dataplane"
	v3ioerrors "github.com/v3io/v3io-go/pkg/errors"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type claim struct {
	logger                   logger.Logger
	member                   *member
	shardID                  int
	recordBatchChan          chan *RecordBatch
	stopRecordBatchFetchChan chan struct{}
	currentShardLocation     string
}

func newClaim(member *member, shardID int) (*claim, error) {
	return &claim{
		logger:                   member.streamConsumerGroup.logger.GetChild(fmt.Sprintf("claim-%d", shardID)),
		member:                   member,
		shardID:                  shardID,
		recordBatchChan:          make(chan *RecordBatch, member.streamConsumerGroup.config.Claim.RecordBatchChanSize),
		stopRecordBatchFetchChan: make(chan struct{}, 1),
	}, nil
}

func (c *claim) start() error {
	c.logger.DebugWith("Starting claim")

	go func() {
		err := c.fetchRecordBatches(c.stopRecordBatchFetchChan,
			c.member.streamConsumerGroup.config.Claim.RecordBatchFetch.Interval)

		if err != nil {
			c.logger.WarnWith("Failed to fetch record batches", "err", errors.GetErrorStackString(err, 10))
		}
	}()

	go func() {

		// tell the consumer group handler to consume the claim
		c.logger.DebugWith("Calling ConsumeClaim on handler")
		if err := c.member.handler.ConsumeClaim(c.member.session, c); err != nil {
			c.logger.WarnWith("ConsumeClaim returned with error", "err", errors.GetErrorStackString(err, 10))
		}

		if err := c.stop(); err != nil {
			c.logger.WarnWith("Failed to stop claim after consumption", "err", errors.GetErrorStackString(err, 10))
		}
	}()

	return nil
}

func (c *claim) stop() error {
	c.logger.DebugWith("Stopping claim")

	// don't block
	select {
	case c.stopRecordBatchFetchChan <- struct{}{}:
	default:
	}

	return nil
}

func (c *claim) GetStreamPath() string {
	return c.member.streamConsumerGroup.streamPath
}

func (c *claim) GetShardID() int {
	return c.shardID
}

func (c *claim) GetCurrentLocation() string {
	return c.currentShardLocation
}

func (c *claim) GetRecordBatchChan() <-chan *RecordBatch {
	return c.recordBatchChan
}

func (c *claim) fetchRecordBatches(stopChannel chan struct{}, fetchInterval time.Duration) error {
	var err error

	// read initial location. use config if error. might need to wait until shard actually exists
	c.currentShardLocation, err = c.getCurrentShardLocation(c.shardID)
	if err != nil {
		if err == v3ioerrors.ErrStopped {
			return nil
		}

		return errors.Wrap(err, "Failed to get shard location")
	}

	for {
		select {
		case <-time.After(fetchInterval):
			c.currentShardLocation, err = c.fetchRecordBatch(c.currentShardLocation)
			if err != nil {
				c.logger.WarnWith("Failed fetching record batch", "err", errors.GetErrorStackString(err, 10))
				continue
			}

		case <-stopChannel:
			close(c.recordBatchChan)
			c.logger.Debug("Stopping fetch")
			return nil
		}
	}
}

func (c *claim) fetchRecordBatch(location string) (string, error) {
	getRecordsInput := v3io.GetRecordsInput{
		Path:     path.Join(c.member.streamConsumerGroup.streamPath, strconv.Itoa(c.shardID)),
		Location: location,
		Limit:    c.member.streamConsumerGroup.config.Claim.RecordBatchFetch.NumRecordsInBatch,
	}

	response, err := c.member.streamConsumerGroup.container.GetRecordsSync(&getRecordsInput)
	if err != nil {
		return "", errors.Wrapf(err, "Failed fetching record batch: %s", location)
	}

	defer response.Release()

	getRecordsOutput := response.Output.(*v3io.GetRecordsOutput)

	if len(getRecordsOutput.Records) == 0 {
		return getRecordsOutput.NextLocation, nil
	}

	records := make([]v3io.StreamRecord, len(getRecordsOutput.Records))

	for receivedRecordIndex, receivedRecord := range getRecordsOutput.Records {
		record := v3io.StreamRecord{
			ShardID:        &c.shardID,
			Data:           receivedRecord.Data,
			ClientInfo:     receivedRecord.ClientInfo,
			PartitionKey:   receivedRecord.PartitionKey,
			SequenceNumber: receivedRecord.SequenceNumber,
		}

		records[receivedRecordIndex] = record
	}

	recordBatch := RecordBatch{
		Location:     location,
		Records:      records,
		NextLocation: getRecordsOutput.NextLocation,
		ShardID:      c.shardID,
	}

	// write into chunks channel, blocking if there's no space
	c.recordBatchChan <- &recordBatch

	return getRecordsOutput.NextLocation, nil
}

func (c *claim) getCurrentShardLocation(shardID int) (string, error) {

	// get the location from persistency
	currentShardLocation, err := c.member.streamConsumerGroup.getShardLocationFromPersistency(shardID)
	if err != nil && errors.RootCause(err) != ErrShardNotFound {
		return "", errors.Wrap(err, "Failed to get shard location")
	}

	// if shard wasn't found, try again periodically
	if errors.RootCause(err) == ErrShardNotFound {
		for {
			select {

			// TODO: from configuration
			case <-time.After(c.member.streamConsumerGroup.config.SequenceNumber.ShardWaitInterval):

				// get the location from persistency
				currentShardLocation, err = c.member.streamConsumerGroup.getShardLocationFromPersistency(shardID)
				if err != nil {
					if errors.RootCause(err) == ErrShardNotFound {

						// shard doesn't exist yet, try again
						continue
					}

					return "", errors.Wrap(err, "Failed to get shard location")
				}

				return currentShardLocation, nil
			case <-c.stopRecordBatchFetchChan:
				return "", v3ioerrors.ErrStopped
			}
		}
	}

	return currentShardLocation, nil
}
