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

	"github.com/nuclio/nuclio-sdk"
	"github.com/spf13/viper"
)

type factory struct{}

func (f *factory) Create(parentLogger nuclio.Logger,
	eventSourceConfiguration *viper.Viper,
	runtimeConfiguration *viper.Viper) (eventsource.EventSource, error) {

	// create logger parent
	kinesisLogger := parentLogger.GetChild("kinesis").(nuclio.Logger)

	// get shard configuration
	shards := eventSourceConfiguration.GetStringSlice("shards")

	// create worker allocator
	workerAllocator, err := worker.WorkerFactorySingleton.CreateFixedPoolWorkerAllocator(kinesisLogger,
		len(shards),
		runtimeConfiguration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	// finally, create the event source
	kinesisEventSource, err := newEventSource(kinesisLogger,
		workerAllocator,
		&Configuration{
			Configuration:      *eventsource.NewConfiguration(eventSourceConfiguration),
			awsAccessKeyID:     eventSourceConfiguration.GetString("access_key_id"),
			awsSecretAccessKey: eventSourceConfiguration.GetString("secret_access_key"),
			awsRegionName:      eventSourceConfiguration.GetString("region_name"),
			streamName:         eventSourceConfiguration.GetString("stream_name"),
			shards:             shards,
		},
	)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create rabbit-mq event source")
	}

	return kinesisEventSource, nil
}

// register factory
func init() {
	eventsource.RegistrySingleton.Register("kinesis", &factory{})
}
