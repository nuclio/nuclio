package runtime

import "github.com/nuclio/nuclio/cmd/processor/app/event"

type Runtime interface {
	ProcessEvent(event event.Event) (interface{}, error)
}
