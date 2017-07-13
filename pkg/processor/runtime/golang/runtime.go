package golang

import (
	"fmt"

	"github.com/nuclio/nuclio-sdk/event"
	"github.com/nuclio/nuclio-sdk/logger"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/golang/event_handler"

	"github.com/pkg/errors"
)

type golang struct {
	runtime.AbstractRuntime
	configuration *Configuration
	eventHandler  golangruntimeeventhandler.EventHandler
}

func NewRuntime(parentLogger logger.Logger, configuration *Configuration) (runtime.Runtime, error) {
	handlerName := configuration.EventHandlerName

	eventHandler, err := golangruntimeeventhandler.EventHandlers.Get(handlerName)
	if err != nil {
		return nil, err
	}

	// create the command string
	newGoRuntime := &golang{
		AbstractRuntime: *runtime.NewAbstractRuntime(parentLogger.GetChild("golang").(logger.Logger), &configuration.Configuration),
		configuration:   configuration,
		eventHandler:    eventHandler.(golangruntimeeventhandler.EventHandler),
	}

	return newGoRuntime, nil
}

func (g *golang) ProcessEvent(event event.Event) (response interface{}, err error) {
	defer func() {
		if perr := recover(); perr != nil {
			response = nil
			// We can't use error.Wrap here since perr is an interface{}
			err = fmt.Errorf("panic in event handler - %s", perr)
		}
	}()

	// call the registered event handler
	response, err = g.eventHandler(g.Context, event)
	if err != nil {
		return nil, errors.Wrap(err, "Event handler returned error")
	}

	return response, nil
}
