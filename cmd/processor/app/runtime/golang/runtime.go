package golang_runtime

import (
	"errors"
	"fmt"

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
	handlers := golang_runtime_event_handler.EventHandlers
	handlerName := configuration.EventHandlerName

	// get event handler by name
	eventHandler, foundEventHandler := handlers[handlerName]
	if !foundEventHandler {
		return nil, errors.New(fmt.Sprintf("Failed to find event handler by name: %s", handlerName))
	}

	// create the command string
	newGoRuntime := &golang{
		logger:        logger.GetChild("golang"),
		configuration: configuration,
		eventHandler:  eventHandler,
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
