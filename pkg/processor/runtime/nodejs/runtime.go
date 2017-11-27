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
	"net/http"
	"os"
	"sync"
	"unsafe"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/nuclio-sdk"
)

/*
#cgo pkg-config: nodejs

#include <stdlib.h> // for free
#include <string.h> // for strlen
#include "interface.h"
*/
import "C"

type nodejs struct {
	runtime.AbstractRuntime
	configuration *runtime.Configuration
	worker        unsafe.Pointer
}

var contextPool = sync.Pool{
	New: func() interface{} {
		return &nuclio.Context{}
	},
}

// NewRuntime returns a new nodejs runtime
func NewRuntime(parentLogger nuclio.Logger, configuration *runtime.Configuration) (runtime.Runtime, error) {
	logger := parentLogger.GetChild("nodejs")

	var err error

	abstractRuntime, err := runtime.NewAbstractRuntime(logger, configuration)
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
	result := C.new_worker(C.CString(codeStr), C.CString(configuration.Spec.Handler))
	if result.error_message != nil {
		err := fmt.Sprintf("Can't create node worker - %s\n", C.GoString(result.error_message))
		C.free(unsafe.Pointer(result.error_message))
		return nil, errors.New(err)
	}

	newRuntime.worker = result.worker
	return newRuntime, nil

}

func (node *nodejs) ProcessEvent(event nuclio.Event, functionLogger nuclio.Logger) (interface{}, error) {
	node.Logger.DebugWith("Processing event",
		"name", node.configuration.Meta.Name,
		"version", node.configuration.Spec.Version,
		"eventID", event.GetID())

	context := contextPool.Get().(*nuclio.Context)
	context.Logger = node.resolveFunctionLogger(functionLogger)

	jsResponse := C.handle_event(node.worker, unsafe.Pointer(context), unsafe.Pointer(&event))

	contextPool.Put(context)

	if jsResponse.error_message != nil {
		size := C.int(C.strlen(jsResponse.error_message))
		return nuclio.Response{
			StatusCode:  http.StatusInternalServerError,
			Body:        C.GoBytes(unsafe.Pointer(jsResponse.error_message), size),
			ContentType: "text/plain",
		}, nil
	}

	bodyLength := C.int(C.strlen(jsResponse.body))
	response := nuclio.Response{
		StatusCode:  int(jsResponse.status_code),
		Body:        C.GoBytes(unsafe.Pointer(jsResponse.body), bodyLength),
		ContentType: C.GoString(jsResponse.content_type),
		// TODO: Headers (jsResponse.headers) - see interface.cc
	}

	// TODO: Free fields in response (should we? does v8 clear them?)
	return response, nil
}

func (node *nodejs) readHandlerCode() ([]byte, error) {
	return ioutil.ReadFile(node.handlerFilePath())
}

func (node *nodejs) handlerFilePath() string {
	handlerPath := os.Getenv("NUCLIO_JS_HANDLER")
	if handlerPath == "" {
		return "/opt/nuclio/handler/handler.js"
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
