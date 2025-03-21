package natsjetstream

import (
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nuclio/nuclio-sdk-go"
)


// Event allows access to the NATS message
type Event struct {
	nuclio.AbstractEvent
	natsMessage jetstream.Msg
}

func (e *Event) GetBody() []byte {
	return e.natsMessage.Data()
}

// GetHeaders loads all headers into a map of string / interface{}
func (e *Event) GetHeaders() map[string]interface{} {

	// convert headers to map[string]interface{}
	headers := map[string]interface{}{}
	for key, value := range e.natsMessage.Headers() {
		headers[key] = value
	}
	return headers
}

// func (e *Event) GetPath() string {
// 	return e.natsMessage.Subject
// }

func (e *Event) GetSize() int {
	return len(e.natsMessage.Data())
}

func (e *Event) NATSMessage() jetstream.Msg {
	return e.natsMessage
}
