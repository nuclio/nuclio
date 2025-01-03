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

package kinesis

import (
	"fmt"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	kinesisclient "github.com/sendgridlabs/go-kinesis"
)

var errIteratorExpired = errors.New("IteratorExpired")

type shard struct {
	logger         logger.Logger
	kinesisTrigger *kinesis
	shardID        string
	worker         *worker.Worker
}

func newShard(parentLogger logger.Logger, kinesisTrigger *kinesis, shardID string) (*shard, error) {
	var err error

	newShard := &shard{
		logger:         parentLogger.GetChild(fmt.Sprintf("shard-%s", shardID)),
		kinesisTrigger: kinesisTrigger,
		shardID:        shardID,
	}

	newShard.worker, err = kinesisTrigger.AbstractTrigger.WorkerAllocator.Allocate(0)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to allocate worker")
	}

	return newShard, nil
}

func (s *shard) readFromShard() error {
	var err error

	s.logger.DebugWith("Starting to read from shard",
		"pollingPeriod", s.kinesisTrigger.configuration.PollingPeriod,
		"iteratorType", s.kinesisTrigger.configuration.IteratorType)

	// prepare args for get records
	getRecordArgs := kinesisclient.NewArgs()

	var getRecordsResponse *kinesisclient.GetRecordsResp
	lastRecordSequenceNumber := ""

	for {

		// get next records
		getRecordsResponse, err = s.getNextRecords(getRecordArgs, getRecordsResponse, lastRecordSequenceNumber)

		if err != nil {

			// if there was an error other than iterator expired, wait a bit
			if err != errIteratorExpired {
				s.logger.WarnWith("Failed to get next records", "err", errors.GetErrorStackString(err, 5))
				time.Sleep(s.kinesisTrigger.configuration.pollingPeriodDuration)
			}

			continue
		}

		// if we got records, handle them
		if len(getRecordsResponse.Records) > 0 {
			for _, record := range getRecordsResponse.Records {
				event := Event{
					body: record.Data,
				}

				// process the event, don't really do anything with response
				s.kinesisTrigger.AbstractTrigger.SubmitEventToWorker(nil, s.worker, &event) // nolint: errcheck
			}

			// save last sequence number in the batch. we might need to create a shard iterator at this
			// sequence number
			lastRecordSequenceNumber = getRecordsResponse.Records[len(getRecordsResponse.Records)-1].SequenceNumber

		} else {
			time.Sleep(s.kinesisTrigger.configuration.pollingPeriodDuration)
		}
	}
}

func (s *shard) getNextRecords(getRecordArgs *kinesisclient.RequestArgs,
	getRecordsResponse *kinesisclient.GetRecordsResp,
	lastRecordSequenceNumber string) (*kinesisclient.GetRecordsResp, error) {

	// get the shard iterator
	shardIterator, err := s.getShardIterator(getRecordsResponse, lastRecordSequenceNumber)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get shard iterator")
	}

	// set shard iterator
	getRecordArgs.Add("ShardIterator", shardIterator)

	// try to get records
	getRecordsResponse, err = s.kinesisTrigger.kinesisClient.GetRecords(getRecordArgs)
	if err != nil {

		// if the error denotes an expired iterator, force recreation of iterator by nullifying the
		// records response. getShardIterator() will produce an iterator based off of the last successful
		// read sequence number
		if strings.Contains(err.Error(), "Iterator expired") {
			return nil, errIteratorExpired
		}

		return nil, errors.Wrap(err, "Failed to get records")
	}

	return getRecordsResponse, nil
}

func (s *shard) getShardIterator(lastRecordsResponse *kinesisclient.GetRecordsResp,
	lastRecordSequenceNumber string) (string, error) {

	// if there's a response we need to create the iterator from, use that first
	if lastRecordsResponse != nil {
		return lastRecordsResponse.NextShardIterator, nil
	}

	getShardIteratorArgs := kinesisclient.NewArgs()
	getShardIteratorArgs.Add("StreamName", s.kinesisTrigger.configuration.StreamName)
	getShardIteratorArgs.Add("ShardId", s.shardID)

	if lastRecordSequenceNumber == "" {

		// if there's no records response and no record sequence number, this must be the first time. use
		// the iterator type specified in the configuration
		getShardIteratorArgs.Add("ShardIteratorType", s.kinesisTrigger.configuration.IteratorType)
		s.logger.DebugWith("Creating initial iterator", "type", s.kinesisTrigger.configuration.IteratorType)
	} else {

		// if a sequence number was passed, get a shard iterator at that point
		getShardIteratorArgs.Add("ShardIteratorType", "AFTER_SEQUENCE_NUMBER")
		getShardIteratorArgs.Add("StartingSequenceNumber", lastRecordSequenceNumber)
		s.logger.DebugWith("Creating iterator at sequence", "seq", lastRecordSequenceNumber)
	}

	getShardIteratorResponse, err := s.kinesisTrigger.kinesisClient.GetShardIterator(getShardIteratorArgs)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get shard iterator")
	}

	return getShardIteratorResponse.ShardIterator, nil
}
