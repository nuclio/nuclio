package rabbitmq

import "github.com/nuclio/nuclio/cmd/processor/app/eventsource"

type Configuration struct {
	eventsource.Configuration
	BrokerUrl          string
	BrokerExchangeName string
}
