package rabbitmq

import "github.com/nuclio/nuclio/cmd/processor/app/event_source"

type Configuration struct {
	eventsource.Configuration
	BrokerUrl          string
	BrokerExchangeName string
}
