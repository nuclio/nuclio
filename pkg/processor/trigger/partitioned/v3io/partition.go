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

package v3io

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/processor/trigger/partitioned"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	v3iohttp "github.com/v3io/v3io-go-http"
)

const NotFoundError = "Failed GET with status 404"

type partition struct {
	*partitioned.AbstractPartition
	partitionID int
	event       Event
	v3ioTrigger *v3io
}

func newPartition(parentLogger logger.Logger, v3ioTrigger *v3io, partitionID int) (*partition, error) {
	var err error

	partitionName := fmt.Sprintf("partition-%d", partitionID)

	// create a partition
	newPartition := &partition{
		partitionID: partitionID,
		v3ioTrigger: v3ioTrigger,
	}

	newPartition.AbstractPartition, err = partitioned.NewAbstractPartition(parentLogger.GetChild(partitionName),
		v3ioTrigger.AbstractStream)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract partition")
	}
	return newPartition, nil
}

func (p *partition) Read() error {
	partitionPath := fmt.Sprintf("%s/%d", strings.TrimRight(p.v3ioTrigger.streamPath, "/"), p.partitionID)
	pollingInterval := time.Duration(p.v3ioTrigger.configuration.PollingIntervalMs) * time.Millisecond

	err := p.waitPartitionAvailable(partitionPath, pollingInterval)
	if err != nil {
		return errors.Wrap(err, "Failed to wait for partition availability")
	}

	location, err := p.seek(partitionPath)
	if err != nil {
		return err
	}

	p.Logger.DebugWith("Starting to read from partition",
		"location", location,
		"pollingInterval", p.v3ioTrigger.configuration.PollingIntervalMs)

	for {
		getRecordsOutput, err := p.getRecords(partitionPath, location, pollingInterval)

		if err != nil {
			p.Logger.ErrorWith("Failed to read from partition", "error", err)
			time.Sleep(pollingInterval)
			continue
		}

		// set next location
		location = getRecordsOutput.NextLocation

		// handle records by processing them in the function
		for _, record := range getRecordsOutput.Records {

			// set the record in the event
			p.event.record = &record

			// submit to worker
			p.Stream.SubmitEventToWorker(nil, p.Worker, &p.event) // nolint: errcheck
		}

		if len(getRecordsOutput.Records) == 0 {
			time.Sleep(pollingInterval)
		}
	}
}

func (p *partition) seek(partitionPath string) (string, error) {
	p.Logger.DebugWith("Seeking partition",
		"partitionPath", partitionPath,
		"seekType", p.v3ioTrigger.seekType)

	response, err := p.v3ioTrigger.container.Sync.SeekShard(&v3iohttp.SeekShardInput{
		Path: partitionPath,
		Type: p.v3ioTrigger.seekType,
	})
	if err != nil {
		return "", errors.Wrap(err, "Failed to seek partition")
	}
	response.Release()

	location := response.Output.(*v3iohttp.SeekShardOutput).Location
	if location == "" {
		return "", errors.New("Got empty location from seek")
	}
	return location, nil
}

func (p *partition) getRecords(partitionPath string, location string, pollingInterval time.Duration) (*v3iohttp.GetRecordsOutput, error) {
	response, err := p.v3ioTrigger.container.Sync.GetRecords(&v3iohttp.GetRecordsInput{
		Path:     partitionPath,
		Location: location,
		Limit:    p.v3ioTrigger.configuration.ReadBatchSize,
	})
	if err != nil {
		return nil, err
	}
	response.Release()

	return response.Output.(*v3iohttp.GetRecordsOutput), nil
}

func (p *partition) waitPartitionAvailable(partitionPath string, pollingInterval time.Duration) error {
	for {
		listBucketResult, err := p.getListBucket()

		if err != nil {

			// print only if error is something else other than 404
			if err.Error() != NotFoundError {
				p.Logger.ErrorWith("Failed to list bucket", "err", err)
			}
			time.Sleep(pollingInterval)
			continue
		}

		// look to see if the partition inside the stream path
		for _, partition := range listBucketResult.Contents {
			splittedPartitionKey := strings.Split(strings.TrimRight(partition.Key, "/"), "/")
			if splittedPartitionKey[len(splittedPartitionKey)-1] == strconv.Itoa(p.partitionID) {
				return nil
			}
		}
		time.Sleep(pollingInterval)
	}
}

func (p *partition) getListBucket() (*v3iohttp.ListBucketOutput, error) {
	request := &v3iohttp.ListBucketInput{
		Path: p.v3ioTrigger.streamPath,
	}
	response, err := p.v3ioTrigger.container.Sync.ListBucket(request)

	// upon errors, response is being released by ListBucket
	if err != nil {
		return nil, err
	}
	response.Release()

	listBucketResult, ok := response.Output.(*v3iohttp.ListBucketOutput)
	if !ok {
		return nil, errors.New("Failed to cast response to ListBucketOutput")
	}

	return listBucketResult, nil
}
