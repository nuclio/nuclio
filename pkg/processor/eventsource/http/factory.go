package http

import (
	"github.com/nuclio/nuclio-sdk/logger"
	"github.com/nuclio/nuclio/pkg/processor/eventsource"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type factory struct{}

func (f *factory) Create(parentLogger logger.Logger,
	eventSourceConfiguration *viper.Viper,
	runtimeConfiguration *viper.Viper) (eventsource.EventSource, error) {

	// defaults
	eventSourceConfiguration.SetDefault("num_workers", 1)
	eventSourceConfiguration.SetDefault("listen_address", ":1967")

	// create logger parent
	httpLogger := parentLogger.GetChild("http").(logger.Logger)

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
			eventSourceConfiguration.GetString("listen_address"),
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
