package generator

import "github.com/nuclio/nuclio/cmd/processor/app/event_source"

type Configuration struct {
	event_source.Configuration
	NumWorkers int
	MinDelayMs int
	MaxDelayMs int
}
