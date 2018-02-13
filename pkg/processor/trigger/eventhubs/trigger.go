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

package eventhubs

import (
	"fmt"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/logger"
	ehclient "github.com/nuclio/amqp"
)

type eventhubs struct {
	trigger.AbstractTrigger
	event         Event
	configuration *Configuration
	ehClient      ehclient.Client
	session       ehclient.Session
	partitions    []*partition
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {

	newTrigger := &eventhubs{
		AbstractTrigger: trigger.AbstractTrigger{
			ID:              configuration.ID,
			Logger:          parentLogger.GetChild(configuration.ID),
			WorkerAllocator: workerAllocator,
			Class:           "async",
			Kind:            "eventhubs",
		},
		configuration: configuration,
	}
	client, err := ehclient.Dial(fmt.Sprintf("amqps://%s.servicebus.windows.net", configuration.Namespace),
		ehclient.ConnSASLPlain(configuration.SharedAccessKeyName, configuration.SharedAccessKeyValue),
	)

	if err != nil {
		errors.Wrap(err, "Dialing AMQP server:")
	}

	newTrigger.ehClient = *client
	session, err := client.NewSession()
	if err != nil {
		errors.Wrap(err, "Creating AMQP session:")
	}

	newTrigger.session = *session

	// iterate over partitions and create
	for _, partitionID := range configuration.Partitions {

		// create the partition
		partition, err := newPartition(newTrigger.Logger, newTrigger, partitionID)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create partition")
		}

		// add partition
		newTrigger.partitions = append(newTrigger.partitions, partition)
	}

	return newTrigger, nil
}

func (e *eventhubs) Start(checkpoint trigger.Checkpoint) error {
	e.Logger.InfoWith("Starting",
		"EventHubName", e.configuration.EventHubName,
		"Partitions", e.configuration.Partitions)

	for _, partitionInstance := range e.partitions {

		// start reading from partition
		go func(partitionInstance *partition) {

			if err := partitionInstance.readFromPartition(); err != nil {
				e.Logger.ErrorWith("Failed to read from partition", "err", err)
			}
		}(partitionInstance)
	}

	return nil
}

func (e *eventhubs) Stop(force bool) (trigger.Checkpoint, error) {
	e.ehClient.Close()
	return nil, nil
}

func (e *eventhubs) GetConfig() map[string]interface{} {
	return common.StructureToMap(e.configuration)
}
