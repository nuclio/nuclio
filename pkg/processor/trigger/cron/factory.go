package cron

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	"github.com/nuclio/nuclio/pkg/errors"
)

type factory struct{}

func (f *factory) Create(parentLogger nuclio.Logger,
	ID string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (trigger.Trigger, error) {

	// create logger parent
	cronLogger := parentLogger.GetChild("cron")

	configuration, err := NewConfiguration(ID, triggerConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse cron trigger configuration")
	}

	workerAllocator, err := worker.WorkerFactorySingleton.CreateSingletonPoolWorkerAllocator(cronLogger,
		runtimeConfiguration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	// finally, create the trigger (only 8080 for now)
	cronTrigger, err := newTrigger(cronLogger,
		workerAllocator,
		configuration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create Cron trigger")
	}

	return cronTrigger, nil
}

// register factory
func init() {
	trigger.RegistrySingleton.Register("cron", &factory{})
}
