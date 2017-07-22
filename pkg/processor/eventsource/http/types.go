package http

import "github.com/nuclio/nuclio/pkg/processor/eventsource"

type Configuration struct {
	eventsource.Configuration
	ListenAddress string
}
