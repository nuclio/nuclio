package golangruntimeeventhandler

import (
	"github.com/nuclio/nuclio-sdk/event"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
)

type EventHandler func(context *runtime.Context, event event.Event) (interface{}, error)
