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

package pypy

/*
#cgo CFLAGS: -I/opt/pypy/include
#cgo LDFLAGS: -lpypy-c

#include "PyPy.h"
#include "interface.h"

#include <stdlib.h> // for free
*/
import "C"

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/nuclio-sdk"
)

var (
	pyCodeTemplate = `
import sys; sys.path.insert(0, '%s')
import nuclio_interface
nuclio_interface.fill_api(c_argument)
`
)

type pypy struct {
	runtime.AbstractRuntime
	configuration *Configuration
}

type pypyResponse struct {
	body         string
	statusCode   int
	contentType  string
	errorMessage string
}

// NewRuntime returns a new Python runtime
func NewRuntime(parentLogger nuclio.Logger, configuration *Configuration) (runtime.Runtime, error) {
	logger := parentLogger.GetChild("python")

	var err error

	abstractRuntime, err := runtime.NewAbstractRuntime(logger, &configuration.Configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Can't create AbstractRuntime")
	}

	newPyPyRuntime := &pypy{
		AbstractRuntime: *abstractRuntime,
		configuration:   configuration,
	}

	if err := newPyPyRuntime.init(); err != nil {
		return nil, err
	}

	return newPyPyRuntime, nil
}

func (py *pypy) init() error {
	C.rpython_startup_code()

	// TODO: From env? (but it's fixed in -I at cgo header above)
	pypyHome := "/opt/pypy"
	if i := C.pypy_setup_home(C.CString(pypyHome), C.int(0)); i != 0 {
		return errors.Errorf("Can't set PyPy home to %q", pypyHome)
	}

	pyCode := fmt.Sprintf(pyCodeTemplate, py.getPythonPath())
	if i := C.pypy_execute_source_ptr(C.CString(pyCode), unsafe.Pointer(&C.api)); i != 0 {
		return errors.Errorf("Can't execute initialization code")
	}

	C.init()
	err := C.set_handler(C.CString(py.configuration.Handler))
	defer C.free(unsafe.Pointer(err))

	output := C.GoString(err)
	if output != "" {
		return errors.Errorf("Can't set handler %q - %s", py.configuration.Handler, output)
	}

	return nil
}

func (py *pypy) ProcessEvent(event nuclio.Event, functionLogger nuclio.Logger) (interface{}, error) {
	py.Logger.DebugWith("Processing event",
		"name", py.configuration.Name,
		"version", py.configuration.Version,
		"eventID", event.GetID())

	cResponse := C.handle_event(unsafe.Pointer(&event))
	response := py.responseToGo(cResponse)

	if response.errorMessage != "" {
		return nil, errors.New(response.errorMessage)
	}

	return nuclio.Response{
		StatusCode:  response.statusCode,
		ContentType: response.contentType,
		Body:        []byte(response.body),
	}, nil
}

func (py *pypy) responseToGo(cResponse *C.response_t) *pypyResponse {
	response := &pypyResponse{}

	response.body = C.GoString(cResponse.body)
	C.free(unsafe.Pointer(cResponse.body))

	response.contentType = C.GoString(cResponse.content_type)
	C.free(unsafe.Pointer(cResponse.content_type))

	response.errorMessage = C.GoString(cResponse.error)
	C.free(unsafe.Pointer(cResponse.error))

	response.statusCode = int(cResponse.status_code)

	C.free(unsafe.Pointer(cResponse))

	return response
}

// TODO: Global processor configuration, where should this go?
func (py *pypy) getPythonPath() string {
	pythonPath := os.Getenv("NUCLIO_PYTHON_PATH")
	if len(pythonPath) == 0 {
		return "/opt/nuclio"
	}

	return pythonPath
}
