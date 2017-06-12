package rabbit_mq

import "github.com/nuclio/nuclio/cmd/processor/app/event_source"

type Configuration struct {
	event_source.Configuration
	BrokerUrl                  string
	BrokerExchangeName         string
}
