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

package kafka

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/eventsource"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/nuclio-sdk"
	"github.com/spf13/viper"
	"github.com/nuclio/nuclio/pkg/util/common"
)

type factory struct{}

func (f *factory) Create(parentLogger nuclio.Logger,
	eventSourceConfiguration *viper.Viper,
	runtimeConfiguration *viper.Viper) (eventsource.EventSource, error) {

	// create logger parent
	kafkaLogger := parentLogger.GetChild("kafka").(nuclio.Logger)

	// get partition configuration
	partitions := eventSourceConfiguration.GetStringSlice("partitions")

	// create worker allocator
	workerAllocator, err := worker.WorkerFactorySingleton.CreateFixedPoolWorkerAllocator(kafkaLogger,
		len(partitions),
		runtimeConfiguration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	// finally, create the event source
	kafkaConfiguration := &Configuration{
		Configuration: *eventsource.NewConfiguration(eventSourceConfiguration),
		Host: eventSourceConfiguration.GetString("host"),
		Topic: eventSourceConfiguration.GetString("topic"),
	}

	if kafkaConfiguration.Partitions, err = common.StringSliceToIntSlice(partitions); err != nil {
		return nil, errors.Wrap(err, "Kafka partitions contains invalid values")
	}

	eventSource, err := newEventSource(kafkaLogger, workerAllocator, kafkaConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create kafka event source")
	}

	kafkaLogger.DebugWith("Created kafaka event source", "config", kafkaConfiguration)
	return eventSource, nil
}

// register factory
func init() {
	eventsource.RegistrySingleton.Register("kafka", &factory{})
}
