/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
