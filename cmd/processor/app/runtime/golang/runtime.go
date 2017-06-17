package golang

import (
	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"github.com/nuclio/nuclio/cmd/processor/app/runtime"
	"github.com/nuclio/nuclio/cmd/processor/app/runtime/golang/event_handler"
	"github.com/nuclio/nuclio/pkg/logger"
)

type golang struct {
	logger        logger.Logger
	configuration *Configuration
	eventHandler  golang_runtime_event_handler.EventHandler
}

func NewRuntime(logger logger.Logger, configuration *Configuration) (runtime.Runtime, error) {
	handlerName := configuration.EventHandlerName

	eventHandler, err := golang_runtime_event_handler.EventHandlers.Get(handlerName)
	if err != nil {
		return nil, err
	}

	// create the command string
	newGoRuntime := &golang{
		logger:        logger.GetChild("golang"),
		configuration: configuration,
		eventHandler:  eventHandler.(golang_runtime_event_handler.EventHandler),
	}

	return newGoRuntime, nil
}

func (g *golang) ProcessEvent(event event.Event) (interface{}, error) {

	// call the registered event handler
	response, err := g.eventHandler(event)
	if err != nil {
		return nil, g.logger.Report(err, "Event handler returned error")
	}

	return response, nil
}
