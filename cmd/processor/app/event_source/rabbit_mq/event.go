package rabbit_mq

import (
	"github.com/streadway/amqp"

	"github.com/nuclio/nuclio/cmd/processor/app/event"
)

// allows accessing an amqp.Delivery
type Event struct {
	event.AbstractSync
	message *amqp.Delivery
}

func (e *Event) GetContentType() string {
	return e.message.ContentType
}

func (e *Event) GetBody() []byte {
	return e.message.Body
}

func (e *Event) GetHeader(key string) []byte {

	value, found := e.message.Headers[key]
	if !found {
		return nil
	}

	switch typedValue := value.(type) {
	case string:
		return []byte(typedValue)
	case []byte:
		return typedValue
	default:
		return nil
	}
}
