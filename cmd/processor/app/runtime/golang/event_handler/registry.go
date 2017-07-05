package golangruntimeeventhandler

import (
	"github.com/nuclio/nuclio/pkg/util/registry"
)

type EventHandlerRegistry struct {
	registry.Registry
}

var EventHandlers = EventHandlerRegistry{
	Registry: *registry.NewRegistry("event_handler"),
}

func (ehr *EventHandlerRegistry) Add(name string, eventHandler EventHandler) {
	ehr.Register(name, eventHandler)
}
