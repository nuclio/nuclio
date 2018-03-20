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

package cron

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/logger"
)

type factory struct{}

func (f *factory) Create(parentLogger logger.Logger,
	ID string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (trigger.Trigger, error) {

	// create logger parent
	cronLogger := parentLogger.GetChild("cron")

	configuration, err := NewConfiguration(ID, triggerConfiguration, runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse cron trigger configuration")
	}

	workerAllocator, err := worker.WorkerFactorySingleton.CreateSingletonPoolWorkerAllocator(cronLogger,
		runtimeConfiguration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	// finally, create the trigger (only 8080 for now)
	cronTrigger, err := newTrigger(cronLogger,
		workerAllocator,
		configuration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create cron trigger")
	}

	return cronTrigger, nil
}

// register factory
func init() {
	trigger.RegistrySingleton.Register("cron", &factory{})
}
