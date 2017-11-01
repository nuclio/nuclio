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

package http

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

	// defaults
	triggerConfiguration.SetDefault("maxWorkers", 1)
	triggerConfiguration.SetDefault("attributes.listenAddress", ":8080")

	// get listen address
	listenAddress := triggerConfiguration.GetString("attributes.listenAddress")

	// create logger parent
	httpLogger := parentLogger.GetChild("http")

	// get how many workers are required
	numWorkers := triggerConfiguration.GetInt("maxWorkers")

	// create worker allocator
	workerAllocator, err := worker.WorkerFactorySingleton.CreateFixedPoolWorkerAllocator(httpLogger,
		numWorkers,
		runtimeConfiguration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	// finally, create the trigger
	httpTrigger, err := newTrigger(httpLogger,
		workerAllocator,
		&Configuration{
			*trigger.NewConfiguration(triggerConfiguration),
			listenAddress,
		})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create HTTP trigger")
	}

	return httpTrigger, nil
}

// register factory
func init() {
	trigger.RegistrySingleton.Register("http", &factory{})
}
