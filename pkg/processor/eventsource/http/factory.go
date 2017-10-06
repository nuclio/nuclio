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
	"net"
	"fmt"
	"strconv"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/eventsource"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/nuclio-sdk"
	"github.com/spf13/viper"
)

const (
	allowedHTTPPort = 8080
)

type factory struct{}

func (f *factory) Create(parentLogger nuclio.Logger,
	eventSourceConfiguration *viper.Viper,
	runtimeConfiguration *viper.Viper) (eventsource.EventSource, error) {

	// defaults
	eventSourceConfiguration.SetDefault("num_workers", 1)
	eventSourceConfiguration.SetDefault("listen_address", ":8080")

	// get listen address
	listenAddress := eventSourceConfiguration.GetString("listen_address")
	_, port, err := net.SplitHostPort(listenAddress)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse listen address")
	}

	// for now, we only support port 8080, since this requires a lot of configuration in lots
	// of places for no real benefit
	if port != strconv.Itoa(allowedHTTPPort) {
		return nil, fmt.Errorf("Only port %d is supported", allowedHTTPPort)
	}

	// create logger parent
	httpLogger := parentLogger.GetChild("http").(nuclio.Logger)

	// get how many workers are required
	numWorkers := eventSourceConfiguration.GetInt("num_workers")

	// create worker allocator
	workerAllocator, err := worker.WorkerFactorySingleton.CreateFixedPoolWorkerAllocator(httpLogger,
		numWorkers,
		runtimeConfiguration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	// finally, create the event source
	httpEventSource, err := newEventSource(httpLogger,
		workerAllocator,
		&Configuration{
			*eventsource.NewConfiguration(eventSourceConfiguration),
			listenAddress,
		})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create HTTP event source")
	}

	return httpEventSource, nil
}

// register factory
func init() {
	eventsource.RegistrySingleton.Register("http", &factory{})
}
