package golangruntimeeventhandler

import (
	"github.com/nuclio/nuclio-sdk"
)

func demo(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	return nil, nil
}

// uncomment to register demo
//func init() {
// 	EventHandlers.Add("demo", demo)
//}
