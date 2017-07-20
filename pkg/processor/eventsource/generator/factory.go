package generator

import (
	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/eventsource"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type factory struct{}

func (f *factory) Create(parentLogger nuclio.Logger,
	eventSourceConfiguration *viper.Viper,
	runtimeConfiguration *viper.Viper) (eventsource.EventSource, error) {

	// defaults
	eventSourceConfiguration.SetDefault("num_workers", "1")
	eventSourceConfiguration.SetDefault("min_delay_ms", "3000")
	eventSourceConfiguration.SetDefault("max_delay_ms", "3000")

	// get how many workers are required
	numWorkers := eventSourceConfiguration.GetInt("num_workers")

	// create logger parent
	generatorLogger := parentLogger.GetChild("generator").(nuclio.Logger)

	// create worker allocator
	workerAllocator, err := worker.WorkerFactorySingleton.CreateFixedPoolWorkerAllocator(generatorLogger,
		numWorkers,
		runtimeConfiguration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	// finally, create the event source
	generatorEventSource, err := newEventSource(generatorLogger,
		workerAllocator,
		&Configuration{
			*eventsource.NewConfiguration(eventSourceConfiguration),
			numWorkers,
			eventSourceConfiguration.GetInt("min_delay_ms"),
			eventSourceConfiguration.GetInt("max_delay_ms"),
		})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create generator event source")
	}

	return generatorEventSource, nil
}

// register factory
func init() {
	eventsource.RegistrySingleton.Register("generator", &factory{})
}
