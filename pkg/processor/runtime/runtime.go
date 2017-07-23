package runtime

import (
	"github.com/nuclio/nuclio-sdk"
)

type Runtime interface {
	ProcessEvent(event nuclio.Event) (interface{}, error)
}

type AbstractRuntime struct {
	Logger  nuclio.Logger
	Context *nuclio.Context
}

func NewAbstractRuntime(logger nuclio.Logger, configuration *Configuration) *AbstractRuntime {
	return &AbstractRuntime{
		Logger:  logger,
		Context: newContext(logger, configuration),
	}
}
