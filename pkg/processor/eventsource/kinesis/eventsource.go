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

package kinesis

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/eventsource"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	"github.com/nuclio/nuclio/pkg/util/common"

	"github.com/nuclio/nuclio-sdk"
	kinesisclient "github.com/sendgridlabs/go-kinesis"
)

type kinesis struct {
	eventsource.AbstractEventSource
	event         Event
	configuration *Configuration
	kinesisAuth   *kinesisclient.AuthCredentials
	kinesisClient kinesisclient.KinesisClient
	shards        []*shard
}

func newEventSource(parentLogger nuclio.Logger,
	workerAllocator worker.WorkerAllocator,
	configuration *Configuration) (eventsource.EventSource, error) {

	newEventSource := &kinesis{
		AbstractEventSource: eventsource.AbstractEventSource{
			ID:              configuration.ID,
			Logger:          parentLogger.GetChild(configuration.ID).(nuclio.Logger),
			WorkerAllocator: workerAllocator,
			Class:           "async",
			Kind:            "kinesis",
		},
		configuration: configuration,
	}

	newEventSource.kinesisAuth = kinesisclient.NewAuth(configuration.awsAccessKeyID,
		configuration.awsSecretAccessKey,
		"")

	newEventSource.kinesisClient = kinesisclient.New(newEventSource.kinesisAuth, configuration.awsRegionName)

	// iterate over shards and create
	for _, shardID := range configuration.shards {

		// create the shard
		shard, err := newShard(newEventSource.Logger, newEventSource, shardID)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create shard")
		}

		// add shard
		newEventSource.shards = append(newEventSource.shards, shard)
	}

	return newEventSource, nil
}

func (k *kinesis) Start(checkpoint eventsource.Checkpoint) error {
	k.Logger.InfoWith("Starting",
		"streamName", k.configuration.streamName,
		"shards", k.configuration.shards)

	for _, shardInstance := range k.shards {

		// start reading from shard
		go func(shardInstance *shard) {
			if err := shardInstance.readFromShard(); err != nil {
				k.Logger.ErrorWith("Failed to read from shard", "err", err)
			}
		}(shardInstance)
	}

	return nil
}

func (k *kinesis) Stop(force bool) (eventsource.Checkpoint, error) {

	// TODO
	return nil, nil
}

func (k *kinesis) GetConfig() map[string]interface{} {
	return common.StructureToMap(k.configuration)
}
