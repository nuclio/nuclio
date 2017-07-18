package golangruntimeeventhandler

import (
	"github.com/nuclio/nuclio-sdk/event"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
)

func demo(context *runtime.Context, event event.Event) (interface{}, error) {
	return nil, nil
}

// uncomment to register demo
// func init() {
// 	EventHandlers.Add("demo", demo)
// }
