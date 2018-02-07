// +build python3

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

package python3

import (
	"fmt"
	"net/http"
	"os"
	goruntime "runtime"
	"strings"
	"unsafe"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

/*
#include <Python.h>
#include <stdlib.h>
#include "types.h"

extern response_t call_handler(unsigned long event_ptr, unsigned long logger_ptr);
extern char *py_last_error();
void free_response_t(response_t response);
*/
import "C"

type python3 struct {
	runtime.AbstractRuntime
	configuration *runtime.Configuration
}

// NewRuntime returns a new Python runtime
func NewRuntime(parentLogger logger.Logger, configuration *runtime.Configuration) (runtime.Runtime, error) {
	loggerInstance := parentLogger.GetChild("python3")

	var err error

	abstractRuntime, err := runtime.NewAbstractRuntime(loggerInstance, configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Can't create AbstractRuntime")
	}

	newRuntime := &python3{
		AbstractRuntime: *abstractRuntime,
		configuration:   configuration,
	}

	if err := newRuntime.initialize(); err != nil {
		return nil, err
	}

	return newRuntime, nil
}

func (py *python3) ProcessEvent(event nuclio.Event, functionLogger logger.Logger) (interface{}, error) {

	py.Logger.DebugWith("Processing event",
		"name", py.configuration.Meta.Name,
		"version", py.configuration.Spec.Version,
		"eventID", event.GetID())

	evtPtr := eventPtr(event)
	functionLogger = py.resolveFunctionLogger(functionLogger)
	logPtr := loggerPtr(functionLogger)
	response := C.call_handler(evtPtr, logPtr)
	goruntime.KeepAlive(event)

	lastError := C.py_last_error()
	if lastError != nil {
		errorMessage := fmt.Sprintf("error in handler: %s", C.GoString(lastError))
		// TODO: C.free(lastError) ?
		return nuclio.Response{
			StatusCode:  http.StatusInternalServerError,
			ContentType: "text/plain",
			Body:        []byte(errorMessage),
		}, nil
	}

	return py.py2GoResponse(response), nil
}

func (py *python3) py2GoResponse(response C.response_t) nuclio.Response {
	size := C.PyBytes_Size(response.body)
	cBody := C.PyBytes_AsString(response.body)
	body := C.GoBytes(unsafe.Pointer(cBody), C.int(size))
	contentType := goString(response.content_type)

	statusCode := int(C.PyLong_AsLongLong(response.status_code))

	headers := make(map[string]interface{})
	vars := varsFromKw(response.headers)
	for i := 0; i < len(vars); i += 2 {
		headers[vars[i].(string)] = vars[i+1]
	}
	C.free_response_t(response)

	return nuclio.Response{
		Body:        body,
		StatusCode:  statusCode,
		ContentType: contentType,
		Headers:     headers,
	}
}

func (py *python3) initialize() error {
	initPython(py.getPythonPath())

	moduleName, handlerName, err := py.parseHandler()
	if err != nil {
		return err
	}
	return loadHandler(moduleName, handlerName)
}

func (py *python3) getPythonPath() string {
	pythonPath := os.Getenv("NUCLIO_PYTHON_PATH")
	if pythonPath == "" {
		return "/opt/nuclio/handler"
	}

	return pythonPath
}

// parseHandler parser the handler to module and handler function name
func (py *python3) parseHandler() (string, string, error) {
	fields := strings.Split(py.configuration.Spec.Handler, ":")
	if len(fields) != 2 {
		return "", "", errors.Errorf("Bad handler - %q", py.configuration.Spec.Handler)
	}

	return fields[0], fields[1], nil
}

// resolveFunctionLogger return either functionLogger if provided or root logger if not
func (py *python3) resolveFunctionLogger(functionLogger logger.Logger) logger.Logger {
	if functionLogger == nil {
		return py.Logger
	}
	return functionLogger
}
