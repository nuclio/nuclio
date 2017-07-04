package http

import (
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/nuclio/nuclio/cmd/processor/app/event_source"
	"github.com/nuclio/nuclio/cmd/processor/app/worker"
	"github.com/nuclio/nuclio/pkg/logger"
)

type factory struct{}

func (f *factory) Create(logger logger.Logger,
	eventSourceConfiguration *viper.Viper,
	runtimeConfiguration *viper.Viper) (event_source.EventSource, error) {

	// defaults
	eventSourceConfiguration.SetDefault("num_workers", 1)
	eventSourceConfiguration.SetDefault("listen_address", ":1967")

	// create logger parent
	httpLogger := logger.GetChild("http")

	// get how many workers are required
	numWorkers := eventSourceConfiguration.GetInt("num_workers")

	// create worker allocator
	workerAllocator, err := worker.WorkerFactorySingleton.CreateFixedPoolWorkerAllocator(httpLogger,
		numWorkers,
		runtimeConfiguration)

	if err != nil {
		return nil, errors.Wrap(nil, "Failed to create worker allocator")
	}

	// finally, create the event source
	httpEventSource, err := newEventSource(httpLogger,
		workerAllocator,
		&Configuration{
			*event_source.NewConfiguration(eventSourceConfiguration),
			eventSourceConfiguration.GetString("listen_address"),
		})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create HTTP event source")
	}

	return httpEventSource, nil
}

// register factory
func init() {
	event_source.RegistrySingleton.Register("http", &factory{})
}
