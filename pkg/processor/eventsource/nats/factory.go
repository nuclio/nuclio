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

package nats

import (
	"runtime"

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
	natsLogger := parentLogger.GetChild("nats").(nuclio.Logger)
	numWorkers := eventSourceConfiguration.GetInt("num_workers")
	if numWorkers == 0 {
		numWorkers = runtime.NumCPU()
	}

	// create worker allocator
	workerAllocator, err := worker.WorkerFactorySingleton.CreateFixedPoolWorkerAllocator(natsLogger,
		numWorkers,
		runtimeConfiguration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	// finally, create the event source
	natsEventSource, err := newEventSource(natsLogger,
		workerAllocator,
		&Configuration{
			Configuration: *eventsource.NewConfiguration(eventSourceConfiguration),
			serverURL:     eventSourceConfiguration.GetString("host_url"),
			topic:         eventSourceConfiguration.GetString("topic"),
		},
	)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create nats event source")
	}

	return natsEventSource, nil
}

// register factory
func init() {
	eventsource.RegistrySingleton.Register("nats", &factory{})
}
