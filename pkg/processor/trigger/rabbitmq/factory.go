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

package rabbitmq

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/nuclio-sdk"
)

type factory struct{}

func (f *factory) Create(parentLogger nuclio.Logger,
	ID string,
	triggerConfiguration *functionconfig.Trigger,
	functionConfiguration *functionconfig.Config) (trigger.Trigger, error) {

	// create logger parent
	rabbitMqLogger := parentLogger.GetChild("rabbit_mq")

	configuration, err := NewConfiguration(ID, triggerConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create configuration")
	}

	// create worker allocator
	workerAllocator, err := worker.WorkerFactorySingleton.CreateSingletonPoolWorkerAllocator(rabbitMqLogger,
		functionConfiguration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	// finally, create the trigger
	rabbitMqTrigger, err := newTrigger(rabbitMqLogger,
		workerAllocator,
		configuration,
	)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create rabbit-mq trigger")
	}

	return rabbitMqTrigger, nil
}

// register factory
func init() {
	trigger.RegistrySingleton.Register("rabbit-mq", &factory{})
}
