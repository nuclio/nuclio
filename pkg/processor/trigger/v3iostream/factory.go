/*
Copyright 2018 The Nuclio Authors.

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

package v3iostream

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
	runtimeConfiguration *runtime.Configuration,
	namedWorkerAllocators map[string]worker.Allocator) (trigger.Trigger, error) {
	var triggerInstance trigger.Trigger

	// create logger parent
	triggerLogger := parentLogger.GetChild("v3io-stream")

	configuration, err := NewConfiguration(ID, triggerConfiguration, runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create configuration")
	}

	// create worker allocator
	workerAllocator, err := worker.WorkerFactorySingleton.CreateFixedPoolWorkerAllocator(triggerLogger,
		configuration.MaxWorkers, // TODO: Allocate dynamically.
		runtimeConfiguration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	triggerInstance, err = newTrigger(triggerLogger, workerAllocator, configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create v3io stream trigger")
	}

	if err := triggerInstance.Initialize(); err != nil {
		return nil, errors.Wrap(err, "Failed to initialize v3io-stream trigger")
	}

	triggerLogger.DebugWith("Created v3io stream trigger", "config", configuration)
	return triggerInstance, nil
}

// register factory
func init() {
	trigger.RegistrySingleton.Register("v3ioStream", &factory{})
}
