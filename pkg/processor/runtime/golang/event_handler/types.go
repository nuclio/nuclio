package golangruntimeeventhandler

import (
	"github.com/nuclio/nuclio-sdk"
)

type EventHandler func(context *nuclio.Context, event nuclio.Event) (interface{}, error)
