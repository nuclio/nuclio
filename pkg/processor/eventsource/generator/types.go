package generator

import "github.com/nuclio/nuclio/pkg/processor/eventsource"

type Configuration struct {
	eventsource.Configuration
	NumWorkers int
	MinDelayMs int
	MaxDelayMs int
}
