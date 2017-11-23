// +build nodejs

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

package nodejs

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"unsafe"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/nuclio-sdk"
)

/*
#cgo pkg-config: nodejs

#include <string.h> // for strlen
#include "interface.h"
*/
import "C"

type nodejs struct {
	runtime.AbstractRuntime
	configuration *Configuration
	worker        unsafe.Pointer
}

var contextPool = sync.Pool{
	New: func() interface{} {
		return &nuclio.Context{}
	},
}

// NewRuntime returns a new nodejs runtime
func NewRuntime(parentLogger nuclio.Logger, configuration *Configuration) (runtime.Runtime, error) {
	logger := parentLogger.GetChild("nodejs")

	var err error

	abstractRuntime, err := runtime.NewAbstractRuntime(logger, &configuration.Configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Can't create AbstractRuntime")
	}

	newRuntime := &nodejs{
		AbstractRuntime: *abstractRuntime,
		configuration:   configuration,
	}

	code, err := newRuntime.readHandlerCode()
	if err != nil {
		return nil, errors.Wrapf(err, "Can't read handler code from %q", newRuntime.handlerFilePath())
	}

	codeStr := string(code)
	C.initialize()
	result := C.new_worker(C.CString(codeStr), C.CString(configuration.Handler))
	if result.error_message != nil {
		err := fmt.Sprintf("Can't create node worker - %s\n", C.GoString(result.error_message))
		return nil, errors.New(err)
	}

	newRuntime.worker = result.worker
	return newRuntime, nil

}

func (node *nodejs) ProcessEvent(event nuclio.Event, functionLogger nuclio.Logger) (interface{}, error) {
	node.Logger.DebugWith("Processing event",
		"name", node.configuration.Name,
		"version", node.configuration.Version,
		"eventID", event.GetID())

	context := contextPool.Get().(*nuclio.Context)
	context.Logger = node.resolveFunctionLogger(functionLogger)

	jsResponse := C.handle_event(node.worker, unsafe.Pointer(context), unsafe.Pointer(&event))

	contextPool.Put(context)

	if jsResponse.error_message != nil {
		return nil, errors.New(C.GoString(jsResponse.error_message))
	}

	bodyLength := C.int(C.strlen(jsResponse.body))
	return nuclio.Response{
		StatusCode:  int(jsResponse.status_code),
		Body:        C.GoBytes(unsafe.Pointer(jsResponse.body), bodyLength),
		ContentType: C.GoString(jsResponse.content_type),
		// TODO: Headers (jsResponse.headers) - see interface.cc
	}, nil
}

func (node *nodejs) readHandlerCode() ([]byte, error) {
	return ioutil.ReadFile(node.handlerFilePath())
}

func (node *nodejs) handlerFilePath() string {
	handlerPath := os.Getenv("NUCLIO_JS_HANDLER")
	if handlerPath == "" {
		return "/opt/nuclio/handler.js"
	}
	return handlerPath
}

// resolveFunctionLogger return either functionLogger if provided or root logger if not
func (node *nodejs) resolveFunctionLogger(functionLogger nuclio.Logger) nuclio.Logger {
	if functionLogger == nil {
		return node.Logger
	}
	return functionLogger
}
