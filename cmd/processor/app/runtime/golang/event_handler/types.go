package golangruntimeeventhandler

import (
	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"github.com/nuclio/nuclio/cmd/processor/app/runtime"
)

type EventHandler func(context *runtime.Context, event event.Event) (interface{}, error)
