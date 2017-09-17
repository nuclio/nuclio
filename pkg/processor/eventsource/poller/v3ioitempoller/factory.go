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

package v3ioitempoller

import (
	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/eventsource"
	"github.com/nuclio/nuclio/pkg/processor/eventsource/poller"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/spf13/viper"
)

type factory struct{}

func (f *factory) Create(parentLogger nuclio.Logger,
	eventSourceConfiguration *viper.Viper,
	runtimeConfiguration *viper.Viper) (eventsource.EventSource, error) {

	// defaults
	eventSourceConfiguration.SetDefault("num_workers", 1)

	// create logger parent
	v3ioItemPollerLogger := parentLogger.GetChild("v3io_item_poller").(nuclio.Logger)

	// get how many workers are required
	numWorkers := eventSourceConfiguration.GetInt("num_workers")

	// create worker allocator
	workerAllocator, err := worker.WorkerFactorySingleton.CreateFixedPoolWorkerAllocator(v3ioItemPollerLogger,
		numWorkers,
		runtimeConfiguration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	// create a configuration structure
	configuration := Configuration{
		Configuration:  *poller.NewConfiguration(eventSourceConfiguration),
		Restart:        false,
		URL:            eventSourceConfiguration.GetString("url"),
		ContainerID:    eventSourceConfiguration.GetInt("container_id"),
		ContainerAlias: eventSourceConfiguration.GetString("container_alias"),
		Paths:          eventSourceConfiguration.GetStringSlice("paths"),
		Attributes:     eventSourceConfiguration.GetStringSlice("attributes"),
		Queries:        eventSourceConfiguration.GetStringSlice("queries"),
		Suffixes:       eventSourceConfiguration.GetStringSlice("suffixes"),
		Incremental:    eventSourceConfiguration.GetBool("incremental"),
		ShardID:        eventSourceConfiguration.GetInt("shard_id"),
		TotalShards:    eventSourceConfiguration.GetInt("total_shards"),
	}

	// finally, create the event source
	v3ioItemPollerEventSource, err := newEventSource(v3ioItemPollerLogger,
		workerAllocator,
		&configuration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create HTTP event source")
	}

	return v3ioItemPollerEventSource, nil
}

// register factory
func init() {
	eventsource.RegistrySingleton.Register("v3io-item-poller", &factory{})
}
