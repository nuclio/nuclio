package rabbit_mq

import (
	"github.com/spf13/viper"

	"github.com/nuclio/nuclio/cmd/processor/app/event_source"
	"github.com/nuclio/nuclio/cmd/processor/app/worker"
	"github.com/nuclio/nuclio/pkg/logger"
)

type factory struct {}

func (f *factory) Create(logger logger.Logger,
	eventSourceConfiguration *viper.Viper,
	runtimeConfiguration *viper.Viper) (event_source.EventSource, error) {

	// create logger parent
	rabbitMqLogger := logger.GetChild("rabbit_mq")

	// create worker allocator
	workerAllocator, err := worker.WorkerFactorySingleton.CreateSingletonPoolWorkerAllocator(rabbitMqLogger,
		runtimeConfiguration)

	if err != nil {
		return nil, logger.Report(nil, "Failed to create worker allocator")
	}

	// finally, create the event source
	generatorEventSource, err := NewEventSource(rabbitMqLogger,
		workerAllocator,
		&Configuration{
			event_source.Configuration{},
			eventSourceConfiguration.GetString("url"),
			eventSourceConfiguration.GetString("exchange"),
		},
	)
	if err != nil {
		return nil, logger.Report(err, "Failed to create rabbit-mq event source")
	}

	return generatorEventSource, nil
}

// register factory
func init() {
	event_source.FactorySingleton.RegisterKind("rabbit-mq", &factory{})
}
