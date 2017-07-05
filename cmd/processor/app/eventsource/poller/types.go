package poller

import (
	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"github.com/nuclio/nuclio/cmd/processor/app/eventsource"

	"github.com/spf13/viper"
)

type Configuration struct {
	eventsource.Configuration
	IntervalMs     int
	MaxBatchSize   int
	MaxBatchWaitMs int
}

func NewConfiguration(configuration *viper.Viper) *Configuration {
	return &Configuration{
		Configuration:  *eventsource.NewConfiguration(configuration),
		IntervalMs:     configuration.GetInt("interval_ms"),
		MaxBatchSize:   configuration.GetInt("max_batch_size"),
		MaxBatchWaitMs: configuration.GetInt("max_batch_wait_ms"),
	}
}

type Poller interface {
	eventsource.EventSource

	// read new events into a channel
	GetNewEvents(chan event.Event) error

	// handle a set of events that were processed
	PostProcessEvents([]event.Event, []interface{}, []error)
}
