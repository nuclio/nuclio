package golang_runtime

import (
	"github.com/nuclio/nuclio/cmd/processor/app/runtime"
)

type Configuration struct {
	runtime.Configuration
	EventHandlerName string
}
