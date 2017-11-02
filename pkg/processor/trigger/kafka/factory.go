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
	"github.com/nuclio/nuclio/pkg/common"
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
	var triggerInstance trigger.Trigger

	// create logger parent
	kafkaLogger := parentLogger.GetChild("kafka")

	// get partition configuration
	partitions := triggerConfiguration.GetStringSlice("attributes.partitions")

	// create worker allocator
	workerAllocator, err := worker.WorkerFactorySingleton.CreateFixedPoolWorkerAllocator(kafkaLogger,
		len(partitions),
		runtimeConfiguration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	// finally, create the trigger
	kafkaConfiguration := &Configuration{
		Configuration: *trigger.NewConfiguration(triggerConfiguration),
		Host:          triggerConfiguration.GetString("attributes.host"),
		Topic:         triggerConfiguration.GetString("attributes.topic"),
	}

	if kafkaConfiguration.Partitions, err = common.StringSliceToIntSlice(partitions); err != nil {
		return nil, errors.Wrap(err, "Kafka partitions contains invalid values")
	}

	triggerInstance, err = newTrigger(kafkaLogger, workerAllocator, kafkaConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create kafka trigger")
	}

	kafkaLogger.DebugWith("Created kafka trigger", "config", kafkaConfiguration)
	return triggerInstance, nil
}

// register factory
func init() {
	trigger.RegistrySingleton.Register("kafka", &factory{})
}
