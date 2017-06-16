package golang_runtime_event_handler

import "github.com/nuclio/nuclio/cmd/processor/app/event"

type EventHandler func(event event.Event) (interface{}, error)
