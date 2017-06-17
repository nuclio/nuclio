package v3io_item_poller

import (
	"github.com/spf13/viper"

	"github.com/nuclio/nuclio/cmd/processor/app/event_source"
	"github.com/nuclio/nuclio/cmd/processor/app/event_source/poller"
	"github.com/nuclio/nuclio/cmd/processor/app/worker"
	"github.com/nuclio/nuclio/pkg/logger"
)

type factory struct{}

func (f *factory) Create(logger logger.Logger,
	eventSourceConfiguration *viper.Viper,
	runtimeConfiguration *viper.Viper) (event_source.EventSource, error) {

	// defaults
	eventSourceConfiguration.SetDefault("num_workers", 1)

	// create logger parent
	v3ioItemPollerLogger := logger.GetChild("v3io_item_poller")

	// get how many workers are required
	numWorkers := eventSourceConfiguration.GetInt("num_workers")

	// create worker allocator
	workerAllocator, err := worker.WorkerFactorySingleton.CreateFixedPoolWorkerAllocator(v3ioItemPollerLogger,
		numWorkers,
		runtimeConfiguration)

	if err != nil {
		return nil, logger.Report(nil, "Failed to create worker allocator")
	}

	// create a configuration structure
	configuration := Configuration{
		Configuration:  *poller.NewConfiguration(eventSourceConfiguration),
		Restart:        false,
		URL:            eventSourceConfiguration.GetString("url"),
		ContainerID:    eventSourceConfiguration.GetInt("container_id"),
		ContainerAlias: eventSourceConfiguration.GetString("container_alias"),
		Paths:          eventSourceConfiguration.GetStringSlice("paths"),
		Attributes:     eventSourceConfiguration.GetStringSlice("attributes"),
		Queries:        eventSourceConfiguration.GetStringSlice("queries"),
		Suffixes:       eventSourceConfiguration.GetStringSlice("suffixes"),
		Incremental:    eventSourceConfiguration.GetBool("incremental"),
		ShardID:        eventSourceConfiguration.GetInt("shard_id"),
		TotalShards:    eventSourceConfiguration.GetInt("total_shards"),
	}

	// finally, create the event source
	v3ioItemPollerEventSource, err := newEventSource(v3ioItemPollerLogger,
		workerAllocator,
		&configuration)

	if err != nil {
		return nil, logger.Report(err, "Failed to create HTTP event source")
	}

	return v3ioItemPollerEventSource, nil
}

// register factory
func init() {
	event_source.RegistrySingleton.Register("v3io-item-poller", &factory{})
}
