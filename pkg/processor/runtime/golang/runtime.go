package golang

import (
	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/golang/event_handler"

	"github.com/pkg/errors"
)

type golang struct {
	runtime.AbstractRuntime
	configuration *Configuration
	eventHandler  golangruntimeeventhandler.EventHandler
}

func NewRuntime(parentLogger nuclio.Logger, configuration *Configuration) (runtime.Runtime, error) {
	handlerName := configuration.EventHandlerName

	runtimeLogger := parentLogger.GetChild("golang").(nuclio.Logger)

	// if the handler name is not specified, just get the first one
	if handlerName == "" {
		eventKinds := golangruntimeeventhandler.EventHandlers.GetKinds()
		if len(eventKinds) == 0 {
			return nil, errors.New("No handlers registered, can't default to first")
		}

		handlerName = eventKinds[0]

		runtimeLogger.InfoWith("Handler name unspecified, using first", "handler", handlerName)
	}

	eventHandler, err := golangruntimeeventhandler.EventHandlers.Get(handlerName)
	if err != nil {
		return nil, err
	}

	// create the command string
	newGoRuntime := &golang{
		AbstractRuntime: *runtime.NewAbstractRuntime(runtimeLogger, &configuration.Configuration),
		configuration:   configuration,
		eventHandler:    eventHandler.(golangruntimeeventhandler.EventHandler),
	}

	return newGoRuntime, nil
}

func (g *golang) ProcessEvent(event nuclio.Event) (interface{}, error) {

	// call the registered event handler
	response, err := g.eventHandler(g.Context, event)
	if err != nil {
		return nil, errors.Wrap(err, "Event handler returned error")
	}

	return response, nil
}
