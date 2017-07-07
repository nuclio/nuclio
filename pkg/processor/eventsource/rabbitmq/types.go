package rabbitmq

import "github.com/nuclio/nuclio/pkg/processor/eventsource"

type Configuration struct {
	eventsource.Configuration
	BrokerUrl          string
	BrokerExchangeName string
}
