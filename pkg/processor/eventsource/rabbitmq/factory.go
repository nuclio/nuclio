package rabbitmq

import (
	"github.com/nuclio/nuclio-logger/logger"
	"github.com/nuclio/nuclio/pkg/processor/eventsource"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type factory struct{}

func (f *factory) Create(parentLogger logger.Logger,
	eventSourceConfiguration *viper.Viper,
	runtimeConfiguration *viper.Viper) (eventsource.EventSource, error) {

	// create logger parent
	rabbitMqLogger := parentLogger.GetChild("rabbit_mq").(logger.Logger)

	// create worker allocator
	workerAllocator, err := worker.WorkerFactorySingleton.CreateSingletonPoolWorkerAllocator(rabbitMqLogger,
		runtimeConfiguration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	// finally, create the event source
	generatorEventSource, err := newEventSource(rabbitMqLogger,
		workerAllocator,
		&Configuration{
			*eventsource.NewConfiguration(eventSourceConfiguration),
			eventSourceConfiguration.GetString("url"),
			eventSourceConfiguration.GetString("exchange"),
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create rabbit-mq event source")
	}

	return generatorEventSource, nil
}

// register factory
func init() {
	eventsource.RegistrySingleton.Register("rabbit-mq", &factory{})
}
