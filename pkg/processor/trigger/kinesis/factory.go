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
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/nuclio-sdk"
	"github.com/spf13/viper"
)

type factory struct{}

func (f *factory) Create(parentLogger nuclio.Logger,
	triggerConfiguration *viper.Viper,
	runtimeConfiguration *viper.Viper) (trigger.Trigger, error) {

	// create logger parent
	kinesisLogger := parentLogger.GetChild("kinesis")

	// get shard configuration
	shards := triggerConfiguration.GetStringSlice("attributes.shards")

	// create worker allocator
	workerAllocator, err := worker.WorkerFactorySingleton.CreateFixedPoolWorkerAllocator(kinesisLogger,
		len(shards),
		runtimeConfiguration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	// finally, create the trigger
	kinesisTrigger, err := newTrigger(kinesisLogger,
		workerAllocator,
		&Configuration{
			Configuration:      *trigger.NewConfiguration(triggerConfiguration),
			AwsAccessKeyID:     triggerConfiguration.GetString("attributes.accessKeyID"),
			AwsSecretAccessKey: triggerConfiguration.GetString("attributes.secretAccessKey"),
			AwsRegionName:      triggerConfiguration.GetString("attributes.regionName"),
			StreamName:         triggerConfiguration.GetString("attributes.streamName"),
			Shards:             shards,
		},
	)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create kinesis trigger")
	}

	return kinesisTrigger, nil
}

// register factory
func init() {
	trigger.RegistrySingleton.Register("kinesis", &factory{})
}
