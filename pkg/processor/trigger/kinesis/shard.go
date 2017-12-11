package kinesis

import (
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/nuclio-sdk"
	kinesisclient "github.com/sendgridlabs/go-kinesis"
)

type shard struct {
	logger         nuclio.Logger
	kinesisTrigger *kinesis
	shardID        string
	worker         *worker.Worker
}

func newShard(parentLogger nuclio.Logger, kinesisTrigger *kinesis, shardID string) (*shard, error) {
	var err error

	newShard := &shard{
		logger:         parentLogger.GetChild(fmt.Sprintf("shard-%s", shardID)),
		kinesisTrigger: kinesisTrigger,
		shardID:        shardID,
	}

	newShard.worker, err = kinesisTrigger.WorkerAllocator.Allocate(0)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to allocate worker")
	}

	return newShard, nil
}

func (s *shard) readFromShard() error {
	s.logger.DebugWith("Starting to read from shard")

	getShardIteratorArgs := kinesisclient.NewArgs()
	getShardIteratorArgs.Add("StreamName", s.kinesisTrigger.configuration.StreamName)
	getShardIteratorArgs.Add("ShardId", s.shardID)
	getShardIteratorArgs.Add("ShardIteratorType", "TRIM_HORIZON")

	getShardIteratorResponse, err := s.kinesisTrigger.kinesisClient.GetShardIterator(getShardIteratorArgs)
	if err != nil {
		return errors.Wrap(err, "Failed to get shard iterator")
	}

	// prepare args for get records
	getRecordArgs := kinesisclient.NewArgs()
	getRecordArgs.Add("ShardIterator", getShardIteratorResponse.ShardIterator)

	for {

		// wait a bit
		time.Sleep(500 * time.Millisecond)

		// try to get records
		getRecordsResponse, err := s.kinesisTrigger.kinesisClient.GetRecords(getRecordArgs)
		if err != nil {
			s.logger.ErrorWith("Failed to get records", "err", err)

			continue
		}

		// if we got records, handle them
		if len(getRecordsResponse.Records) > 0 {
			for _, record := range getRecordsResponse.Records {

				// TODO: event pool
				event := Event{
					body: record.Data,
				}

				// process the event, don't really do anything with response
				s.kinesisTrigger.SubmitEventToWorker(nil, s.worker, &event)
			}
		}

		// set iterator to next
		getRecordArgs.Add("ShardIterator", getRecordsResponse.NextShardIterator)
	}
}
