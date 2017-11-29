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
	"encoding/json"
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

	C.initialize()

	worker, err := newRuntime.createWorker(code, configuration.Spec.Handler)
	if err != nil {
		return nil, err
	}

	newRuntime.worker = worker
	return newRuntime, nil

}

func (node *nodejs) createWorker(code []byte, handler string) (unsafe.Pointer, error) {
	cCode := C.CBytes(code)
	cHandler := C.CString(handler)

	result := C.new_worker((*C.char)(cCode), cHandler)

	C.free(cCode)
	C.free(unsafe.Pointer(cHandler))

	if result.error_message != nil {
		err := fmt.Sprintf("Can't create node worker - %s\n", C.GoString(result.error_message))
		C.free(unsafe.Pointer(result.error_message))
		return nil, errors.New(err)
	}

	return result.worker, nil
}

func (node *nodejs) ProcessEvent(event nuclio.Event, functionLogger nuclio.Logger) (interface{}, error) {
	node.Logger.DebugWith("Processing event",
		"name", node.configuration.Meta.Name,
		"version", node.configuration.Spec.Version,
		"eventID", event.GetID())

	context := contextPool.Get().(*nuclio.Context)
	context.Logger = node.resolveFunctionLogger(functionLogger)

	cResponse := C.handle_event(node.worker, unsafe.Pointer(context), unsafe.Pointer(&event))

	contextPool.Put(context)
	response, err := node.parseResponse(cResponse)
	C.free_response(cResponse)

	if err != nil {
		return nuclio.Response{
			StatusCode:  http.StatusInternalServerError,
			Body:        []byte(err.Error()),
			ContentType: "text/plain",
		}, nil
	}

	return *response, nil
}

func (node *nodejs) cpToBytes(cp *C.char) []byte {
	return C.GoBytes(unsafe.Pointer(cp), C.int(C.strlen(cp)))
}

func (node *nodejs) parseHeaders(cHeaders *C.char) (map[string]interface{}, error) {
	if cHeaders == nil {
		return nil, nil
	}

	data := node.cpToBytes(cHeaders)
	var headers map[string]interface{}

	if err := json.Unmarshal(data, &headers); err != nil {
		return nil, errors.Wrap(err, "Can't decode headers as JSON")
	}

	return headers, nil
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

func (node *nodejs) parseResponse(cResponse C.response_t) (*nuclio.Response, error) {
	if cResponse.error_message != nil {
		return nil, errors.New(C.GoString(cResponse.error_message))
	}

	response := &nuclio.Response{
		Body:        node.cpToBytes(cResponse.body),
		ContentType: C.GoString(cResponse.content_type),
		StatusCode:  int(cResponse.status_code),
	}

	if cResponse.headers != nil {
		data := node.cpToBytes(cResponse.headers)
		var headers map[string]interface{}
		if err := json.Unmarshal(data, &headers); err != nil {
			return nil, errors.Wrap(err, "Can't decode headers as JSON")
		}

		response.Headers = headers
	}

	return response, nil
}
