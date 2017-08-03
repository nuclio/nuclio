package golang

import (
	"log"
	"testing"

	nuclio "github.com/nuclio/nuclio-sdk"
	golangruntimeeventhandler "github.com/nuclio/nuclio/pkg/processor/runtime/golang/event_handler"
	nucliozap "github.com/nuclio/nuclio/pkg/zap"
)

func panicHandler(ctx *nuclio.Context, event nuclio.Event) (interface{}, error) {
	panic("where are my keys?")
}

func TestHandlerPanic(t *testing.T) {
	logger, err := nucliozap.NewNuclioZap("test", nucliozap.DebugLevel)
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
	evt := &nuclio.AbstractSync{}
	_, err = rt.ProcessEvent(evt)
	if err == nil {
		t.Fatalf("no nil error in panic")
	}
}
