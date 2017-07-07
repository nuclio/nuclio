package golang

import (
	"github.com/nuclio/nuclio/pkg/processor/runtime"
)

type Configuration struct {
	runtime.Configuration
	EventHandlerName string
}
