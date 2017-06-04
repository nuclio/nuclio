package golang_runtime_event_handler

import (
	"github.com/nuclio/nuclio/cmd/processor/app/event"
)

// an event handler is specific to the Golang runtime. Golang runtimes
// can call functions who implement this signature - one function
// per worker
type EventHandler func(event event.Event) (interface{}, error)

// register all event handlers
var EventHandlers = map[string]EventHandler{
	"demo": demo,
}
