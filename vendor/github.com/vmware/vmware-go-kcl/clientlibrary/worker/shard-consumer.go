/*
 * Copyright (c) 2018 VMware, Inc.
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of this software and
 * associated documentation files (the "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is furnished to do
 * so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all copies or substantial
 * portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT
 * NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
 * IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
 * WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
 * SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */
// The implementation is derived from https://github.com/patrobinson/gokini
//
// Copyright 2018 Patrick robinson
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
package worker

import (
	log "github.com/sirupsen/logrus"
	"math"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/aws/aws-sdk-go/service/kinesis/kinesisiface"

	chk "github.com/vmware/vmware-go-kcl/clientlibrary/checkpoint"
	"github.com/vmware/vmware-go-kcl/clientlibrary/config"
	kcl "github.com/vmware/vmware-go-kcl/clientlibrary/interfaces"
	"github.com/vmware/vmware-go-kcl/clientlibrary/metrics"
	par "github.com/vmware/vmware-go-kcl/clientlibrary/partition"
)

const (
	// This is the initial state of a shard consumer. This causes the consumer to remain blocked until the all
	// parent shards have been completed.
	WAITING_ON_PARENT_SHARDS ShardConsumerState = iota + 1

	// This state is responsible for initializing the record processor with the shard information.
	INITIALIZING

	//
	PROCESSING

	SHUTDOWN_REQUESTED

	SHUTTING_DOWN

	SHUTDOWN_COMPLETE

	// ErrCodeKMSThrottlingException is defined in the API Reference https://docs.aws.amazon.com/sdk-for-go/api/service/kinesis/#Kinesis.GetRecords
	// But it's not a constant?
	ErrCodeKMSThrottlingException = "KMSThrottlingException"
)

type ShardConsumerState int

// ShardConsumer is responsible for consuming data records of a (specified) shard.
// Note: ShardConsumer only deal with one shard.
type ShardConsumer struct {
	streamName      string
	shard           *par.ShardStatus
	kc              kinesisiface.KinesisAPI
	checkpointer    chk.Checkpointer
	recordProcessor kcl.IRecordProcessor
	kclConfig       *config.KinesisClientLibConfiguration
	stop            *chan struct{}
	waitGroup       *sync.WaitGroup
	consumerID      string
	mService        metrics.MonitoringService
	state           ShardConsumerState
}

func (sc *ShardConsumer) getShardIterator(shard *par.ShardStatus) (*string, error) {
	// Get checkpoint of the shard from dynamoDB
	err := sc.checkpointer.FetchCheckpoint(shard)
	if err != nil && err != chk.ErrSequenceIDNotFound {
		return nil, err
	}

	// If there isn't any checkpoint for the shard, use the configuration value.
	if shard.Checkpoint == "" {
		initPos := sc.kclConfig.InitialPositionInStream
		log.Debugf("No checkpoint recorded for shard: %v, starting with: %v", shard.ID,
			aws.StringValue(config.InitalPositionInStreamToShardIteratorType(initPos)))
		shardIterArgs := &kinesis.GetShardIteratorInput{
			ShardId:           &shard.ID,
			ShardIteratorType: config.InitalPositionInStreamToShardIteratorType(initPos),
			StreamName:        &sc.streamName,
		}
		iterResp, err := sc.kc.GetShardIterator(shardIterArgs)
		if err != nil {
			return nil, err
		}
		return iterResp.ShardIterator, nil
	}

	log.Debugf("Start shard: %v at checkpoint: %v", shard.ID, shard.Checkpoint)
	shardIterArgs := &kinesis.GetShardIteratorInput{
		ShardId:                &shard.ID,
		ShardIteratorType:      aws.String("AFTER_SEQUENCE_NUMBER"),
		StartingSequenceNumber: &shard.Checkpoint,
		StreamName:             &sc.streamName,
	}
	iterResp, err := sc.kc.GetShardIterator(shardIterArgs)
	if err != nil {
		return nil, err
	}
	return iterResp.ShardIterator, nil
}

// getRecords continously poll one shard for data record
// Precondition: it currently has the lease on the shard.
func (sc *ShardConsumer) getRecords(shard *par.ShardStatus) error {
	defer sc.waitGroup.Done()
	defer sc.releaseLease(shard)

	// If the shard is child shard, need to wait until the parent finished.
	if err := sc.waitOnParentShard(shard); err != nil {
		// If parent shard has been deleted by Kinesis system already, just ignore the error.
		if err != chk.ErrSequenceIDNotFound {
			log.Errorf("Error in waiting for parent shard: %v to finish. Error: %+v", shard.ParentShardId, err)
			return err
		}
	}

	shardIterator, err := sc.getShardIterator(shard)
	if err != nil {
		log.Errorf("Unable to get shard iterator for %s: %v", shard.ID, err)
		return err
	}

	// Start processing events and notify record processor on shard and starting checkpoint
	input := &kcl.InitializationInput{
		ShardId:                shard.ID,
		ExtendedSequenceNumber: &kcl.ExtendedSequenceNumber{SequenceNumber: aws.String(shard.Checkpoint)},
	}
	sc.recordProcessor.Initialize(input)

	recordCheckpointer := NewRecordProcessorCheckpoint(shard, sc.checkpointer)
	retriedErrors := 0

	for {
		getRecordsStartTime := time.Now()
		if time.Now().UTC().After(shard.LeaseTimeout.Add(-5 * time.Second)) {
			log.Debugf("Refreshing lease on shard: %s for worker: %s", shard.ID, sc.consumerID)
			err = sc.checkpointer.GetLease(shard, sc.consumerID)
			if err != nil {
				if err.Error() == chk.ErrLeaseNotAquired {
					log.Warnf("Failed in acquiring lease on shard: %s for worker: %s", shard.ID, sc.consumerID)
					return nil
				}
				// log and return error
				log.Errorf("Error in refreshing lease on shard: %s for worker: %s. Error: %+v",
					shard.ID, sc.consumerID, err)
				return err
			}
		}

		log.Debugf("Trying to read %d record from iterator: %v", sc.kclConfig.MaxRecords, aws.StringValue(shardIterator))
		getRecordsArgs := &kinesis.GetRecordsInput{
			Limit:         aws.Int64(int64(sc.kclConfig.MaxRecords)),
			ShardIterator: shardIterator,
		}
		// Get records from stream and retry as needed
		getResp, err := sc.kc.GetRecords(getRecordsArgs)
		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				if awsErr.Code() == kinesis.ErrCodeProvisionedThroughputExceededException || awsErr.Code() == ErrCodeKMSThrottlingException {
					log.Errorf("Error getting records from shard %v: %+v", shard.ID, err)
					retriedErrors++
					// exponential backoff
					// https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Programming.Errors.html#Programming.Errors.RetryAndBackoff
					time.Sleep(time.Duration(math.Exp2(float64(retriedErrors))*100) * time.Millisecond)
					continue
				}
			}
			log.Errorf("Error getting records from Kinesis that cannot be retried: %+v Request: %s", err, getRecordsArgs)
			return err
		}

		// reset the retry count after success
		retriedErrors = 0

		// IRecordProcessorCheckpointer
		input := &kcl.ProcessRecordsInput{
			Records:            getResp.Records,
			MillisBehindLatest: aws.Int64Value(getResp.MillisBehindLatest),
			Checkpointer:       recordCheckpointer,
		}

		recordLength := len(input.Records)
		recordBytes := int64(0)
		log.Debugf("Received %d records, MillisBehindLatest: %v", recordLength, input.MillisBehindLatest)

		for _, r := range getResp.Records {
			recordBytes += int64(len(r.Data))
		}

		if recordLength > 0 || sc.kclConfig.CallProcessRecordsEvenForEmptyRecordList {
			processRecordsStartTime := time.Now()

			// Delivery the events to the record processor
			sc.recordProcessor.ProcessRecords(input)

			// Convert from nanoseconds to milliseconds
			processedRecordsTiming := time.Since(processRecordsStartTime) / 1000000
			sc.mService.RecordProcessRecordsTime(shard.ID, float64(processedRecordsTiming))
		}

		sc.mService.IncrRecordsProcessed(shard.ID, recordLength)
		sc.mService.IncrBytesProcessed(shard.ID, recordBytes)
		sc.mService.MillisBehindLatest(shard.ID, float64(*getResp.MillisBehindLatest))

		// Convert from nanoseconds to milliseconds
		getRecordsTime := time.Since(getRecordsStartTime) / 1000000
		sc.mService.RecordGetRecordsTime(shard.ID, float64(getRecordsTime))

		// Idle between each read, the user is responsible for checkpoint the progress
		// This value is only used when no records are returned; if records are returned, it should immediately
		// retrieve the next set of records.
		if recordLength == 0 && aws.Int64Value(getResp.MillisBehindLatest) < int64(sc.kclConfig.IdleTimeBetweenReadsInMillis) {
			time.Sleep(time.Duration(sc.kclConfig.IdleTimeBetweenReadsInMillis) * time.Millisecond)
		}

		// The shard has been closed, so no new records can be read from it
		if getResp.NextShardIterator == nil {
			log.Infof("Shard %s closed", shard.ID)
			shutdownInput := &kcl.ShutdownInput{ShutdownReason: kcl.TERMINATE, Checkpointer: recordCheckpointer}
			sc.recordProcessor.Shutdown(shutdownInput)
			return nil
		}
		shardIterator = getResp.NextShardIterator

		select {
		case <-*sc.stop:
			shutdownInput := &kcl.ShutdownInput{ShutdownReason: kcl.REQUESTED, Checkpointer: recordCheckpointer}
			sc.recordProcessor.Shutdown(shutdownInput)
			return nil
		case <-time.After(1 * time.Nanosecond):
		}
	}
}

// Need to wait until the parent shard finished
func (sc *ShardConsumer) waitOnParentShard(shard *par.ShardStatus) error {
	if len(shard.ParentShardId) == 0 {
		return nil
	}

	pshard := &par.ShardStatus{
		ID:  shard.ParentShardId,
		Mux: &sync.Mutex{},
	}

	for {
		if err := sc.checkpointer.FetchCheckpoint(pshard); err != nil {
			return err
		}

		// Parent shard is finished.
		if pshard.Checkpoint == chk.SHARD_END {
			return nil
		}

		time.Sleep(time.Duration(sc.kclConfig.ParentShardPollIntervalMillis) * time.Millisecond)
	}
}

// Cleanup the internal lease cache
func (sc *ShardConsumer) releaseLease(shard *par.ShardStatus) {
	log.Infof("Release lease for shard %s", shard.ID)
	shard.SetLeaseOwner("")

	// Release the lease by wiping out the lease owner for the shard
	// Note: we don't need to do anything in case of error here and shard lease will eventuall be expired.
	if err := sc.checkpointer.RemoveLeaseOwner(shard.ID); err != nil {
		log.Errorf("Failed to release shard lease or shard: %s Error: %+v", shard.ID, err)
	}

	// reporting lease lose metrics
	sc.mService.LeaseLost(shard.ID)
}
