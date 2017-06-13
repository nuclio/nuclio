package generator

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
	eventSourceConfiguration.SetDefault("num_workers", "1")
	eventSourceConfiguration.SetDefault("min_delay_ms", "3000")
	eventSourceConfiguration.SetDefault("max_delay_ms", "3000")

	// get how many workers are required
	numWorkers := eventSourceConfiguration.GetInt("num_workers")

	// create logger parent
	generatorLogger := logger.GetChild("generator")

	// create worker allocator
	workerAllocator, err := worker.WorkerFactorySingleton.CreateFixedPoolWorkerAllocator(generatorLogger,
		numWorkers,
		runtimeConfiguration)

	if err != nil {
		return nil, logger.Report(nil, "Failed to create worker allocator")
	}

	// finally, create the event source
	generatorEventSource, err := NewEventSource(generatorLogger,
		workerAllocator,
		&Configuration{
			event_source.Configuration{},
			numWorkers,
			eventSourceConfiguration.GetInt("min_delay_ms"),
			eventSourceConfiguration.GetInt("max_delay_ms"),
		})
	if err != nil {
		return nil, logger.Report(err, "Failed to create generator event source")
	}

	return generatorEventSource, nil
}

// register factory
func init() {
	event_source.RegistrySingleton.RegisterKind("generator", &factory{})
}
