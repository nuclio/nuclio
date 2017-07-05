package generator

import "github.com/nuclio/nuclio/cmd/processor/app/eventsource"

type Configuration struct {
	eventsource.Configuration
	NumWorkers int
	MinDelayMs int
	MaxDelayMs int
}
