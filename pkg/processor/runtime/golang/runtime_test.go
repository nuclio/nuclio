package golang

import (
	"log"
	"testing"

	"github.com/nuclio/nuclio-sdk/event"
	nucliozap "github.com/nuclio/nuclio-zap"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/golang/event_handler"
)

func panicHandler(ctx *runtime.Context, event event.Event) (interface{}, error) {
	panic("where are my keys?")
}

func TestHandlerPanic(t *testing.T) {
	logger, err := nucliozap.NewNuclioZap("test")
	if err != nil {
		log.Fatalf("can't create logger - %s", err)
	}
	name := "panicTestHandler"
	golangruntimeeventhandler.EventHandlers.Add(name, panicHandler)

	cfg := &Configuration{
		EventHandlerName: name,
	}
	rt, err := NewRuntime(logger, cfg)
	if err != nil {
		log.Fatalf("can't create runtime - %s", err)
	}
	evt := &event.AbstractSync{}
	_, err = rt.ProcessEvent(evt)
	if err == nil {
		t.Fatalf("no nil error in panic")
	}
}
