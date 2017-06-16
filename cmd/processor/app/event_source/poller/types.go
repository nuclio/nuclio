package poller

import (
	"github.com/spf13/viper"

	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"github.com/nuclio/nuclio/cmd/processor/app/event_source"
)

type Configuration struct {
	IntervalMs     int
	MaxBatchSize   int
	MaxBatchWaitMs int
}

func NewConfiguration(configuration *viper.Viper) *Configuration {
	return &Configuration{
		IntervalMs:     configuration.GetInt("interval_ms"),
		MaxBatchSize:   configuration.GetInt("max_batch_size"),
		MaxBatchWaitMs: configuration.GetInt("max_batch_wait_ms"),
	}
}

type Poller interface {
	event_source.EventSource

	// read new events into a channel
	GetNewEvents(chan event.Event) error
}
