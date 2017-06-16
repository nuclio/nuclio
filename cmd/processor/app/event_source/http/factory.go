package http

import (
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
		return nil, logger.Report(nil, "Failed to create worker allocator")
	}

	// finally, create the event source
	httpEventSource, err := NewEventSource(httpLogger,
		workerAllocator,
		&Configuration{
			event_source.Configuration{},
			eventSourceConfiguration.GetString("listen_address"),
		})

	if err != nil {
		return nil, logger.Report(err, "Failed to create HTTP event source")
	}

	return httpEventSource, nil
}

// register factory
func init() {
	event_source.RegistrySingleton.Register("http", &factory{})
}
