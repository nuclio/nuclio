package golang

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

// entrypoint is the function which receives events
type entrypoint func(*nuclio.Context, nuclio.Event) (interface{}, error)

// context initializer is the function which is called per runtime to initialize context
type contextInitializer func(*nuclio.Context) error

type handler interface {

	// load will load a handler, given a runtime configuration
	load(*runtime.Configuration) error

	// getEntrypoint returns the entrypoint of the handler
	getEntrypoint() entrypoint

	// getContextInitializer returns the context initializer (if applicable) of the handler
	getContextInitializer() contextInitializer
}

type abstractHandler struct {
	logger             logger.Logger
	entrypoint         entrypoint
	contextInitializer contextInitializer
}

func (ah *abstractHandler) load(configuration *runtime.Configuration) error {

	// if configured, use the built in handler
	if configuration.Spec.Build.Path == "nuclio:builtin" || configuration.Spec.Handler == "nuclio:builtin" {
		ah.logger.WarnWith("Using built in handler, as configured")

		ah.entrypoint = builtInHandler
	}

	return nil
}

// getEntrypoint returns the entrypoint of the handler
func (ah *abstractHandler) getEntrypoint() entrypoint {
	return ah.entrypoint
}

// getContextInitializer returns the context initializer (if applicable) of the handler
func (ah *abstractHandler) getContextInitializer() contextInitializer {
	return ah.contextInitializer
}

func (ah *abstractHandler) parseName(handlerName string) (string, string, error) {

	// if handler is empty, replace with default
	if handlerName == "" {
		handlerName = "main:Handler"
	}

	return functionconfig.ParseHandler(handlerName)
}
