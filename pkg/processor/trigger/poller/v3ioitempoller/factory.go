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
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/trigger/poller"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/nuclio-sdk"
	"github.com/spf13/viper"
)

type factory struct{}

func (f *factory) Create(parentLogger nuclio.Logger,
	triggerConfiguration *viper.Viper,
	runtimeConfiguration *viper.Viper) (trigger.Trigger, error) {

	// defaults
	triggerConfiguration.SetDefault("num_workers", 1)

	// create logger parent
	v3ioItemPollerLogger := parentLogger.GetChild("v3io_item_poller")

	// get how many workers are required
	numWorkers := triggerConfiguration.GetInt("num_workers")

	// create worker allocator
	workerAllocator, err := worker.WorkerFactorySingleton.CreateFixedPoolWorkerAllocator(v3ioItemPollerLogger,
		numWorkers,
		runtimeConfiguration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	// create a configuration structure
	configuration := Configuration{
		Configuration:  *poller.NewConfiguration(triggerConfiguration),
		Restart:        false,
		URL:            triggerConfiguration.GetString("url"),
		ContainerID:    triggerConfiguration.GetInt("container_id"),
		ContainerAlias: triggerConfiguration.GetString("container_alias"),
		Paths:          triggerConfiguration.GetStringSlice("paths"),
		Attributes:     triggerConfiguration.GetStringSlice("attributes"),
		Queries:        triggerConfiguration.GetStringSlice("queries"),
		Suffixes:       triggerConfiguration.GetStringSlice("suffixes"),
		Incremental:    triggerConfiguration.GetBool("incremental"),
		ShardID:        triggerConfiguration.GetInt("shard_id"),
		TotalShards:    triggerConfiguration.GetInt("total_shards"),
	}

	// finally, create the trigger
	v3ioItemPollerTrigger, err := newTrigger(v3ioItemPollerLogger,
		workerAllocator,
		&configuration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create HTTP trigger")
	}

	return v3ioItemPollerTrigger, nil
}

// register factory
func init() {
	trigger.RegistrySingleton.Register("v3io-item-poller", &factory{})
}
