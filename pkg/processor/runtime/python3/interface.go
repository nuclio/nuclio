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
	"os"
	"time"
	"unsafe"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/zap"
)

/*
#cgo pkg-config: python3

#include <Python.h>

#include <stdlib.h>
#include "types.h"

extern void init_python(void);
extern PyObject *new_trigger_info(PyObject *class, PyObject *kind);
extern PyObject *new_logger(unsigned long logger_ptr);
extern PyObject *new_context(PyObject *logger);
extern PyObject *new_datetime(int, int, int, int, int, int, int);
extern int py_type(PyObject *obj);
char *py_type_name(PyObject *obj);
char *py_obj_str(PyObject *obj);
extern int py_is_none(PyObject *obj);
extern int load_handler(char *module_name, char *handler_name);
extern char *py_last_error();
*/
import "C"

var (
	loggerInstance logger.Logger
)

func logError(message string, args ...interface{}) {
	if loggerInstance == nil {
		var err error
		loggerInstance, err = nucliozap.NewNuclioZapCmd("python", nucliozap.ErrorLevel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Can't create logger - %s\n", err)
			fmt.Fprintf(os.Stderr, "\tMESSAGE: %s\n", message)
			fmt.Fprintf(os.Stderr, "\tARGS: %v\n", args)
			return
		}
	}

	loggerInstance.ErrorWith(message, args)
}

// initPython initializes the Python runtime, adding pythonPath to import path
func initPython(pythonPath string) {
	os.Setenv("PYTHONPATH", pythonPath)
	C.init_python()
}

func loadHandler(moduleName, handlerName string) error {
	cModuleName := C.CString(moduleName)
	cHandlerName := C.CString(handlerName)

	if C.load_handler(cModuleName, cHandlerName) != 1 {
		errorMessage := "Can't load handler"
		cError := C.py_last_error()
		if cError != nil {
			errorMessage = fmt.Sprintf("%s: %s", errorMessage, C.GoString(cError))
			//C.free(unsafe.Pointer(cError))
		}
		// TODO: Get error message
		return errors.New(errorMessage)
	}

	//C.free(unsafe.Pointer(cModuleName))
	//C.free(unsafe.Pointer(cHandlerName))

	return nil
}

func pyString(str string) *C.PyObject {
	cStr := C.CString(str)
	obj := C.PyUnicode_FromString(cStr)
	// Python copies the data
	//C.free(unsafe.Pointer(cStr))

	return obj
}

func pyBytes(data []byte) *C.PyObject {
	cData := C.CBytes(data) // Go will allocate data
	obj := C.PyBytes_FromStringAndSize((*C.char)(cData), (C.Py_ssize_t)(len(data)))
	//C.free(cData) // Python copies the data

	return obj
}

func pyDict(m map[string]interface{}) *C.PyObject {
	obj := C.PyDict_New()
	if obj == nil {
		return nil
	}

	for key, value := range m {
		pyKey := pyString(key)
		var pyVal *C.PyObject
		switch value.(type) {
		case int:
			pyVal = C.PyLong_FromLongLong(C.longlong(value.(int)))
		case string:
			pyVal = pyString(value.(string))
		case []byte:
			pyVal = pyBytes(value.([]byte))
		default:
			logError("pyDict - unknown value type: %T", value)
			return nil
		}
		if C.PyDict_SetItem(obj, pyKey, pyVal) == -1 {
			logError("pyDict - unknown value type: %T", value)
			if C.PyErr_Occurred() != nil {
				logError("pyDict - error was %s", pyLastError())
			}
			return nil
		}
	}

	return obj
}

func pyDateTime(t time.Time) *C.PyObject {
	year := C.int(t.Year())
	month := C.int(t.Month())
	day := C.int(t.Day())
	hour := C.int(t.Hour())
	minute := C.int(t.Minute())
	second := C.int(t.Second())
	usec := C.int(t.Nanosecond() / 1000)

	// C.PyDateTime_FromDateAndTime is a macro
	return C.new_datetime(year, month, day, hour, minute, second, usec)
}

func pyLastError() string {
	var ptype, pvalue, ptb *C.PyObject

	C.PyErr_Fetch(&ptype, &pvalue, &ptb)
	if ptype == nil {
		return ""
	}

	cVal := C.py_obj_str(pvalue)
	return C.GoString(cVal)
}

func goString(obj *C.PyObject) string {
	cStr := C.PyUnicode_AsUTF8(obj)
	val := C.GoString(cStr)

	return val
}

func varsFromKw(kw *C.PyObject) []interface{} {
	if (kw == nil) || (C.py_is_none(kw) == 1) {
		return make([]interface{}, 0)
	}

	size := int(C.PyDict_Size(kw))
	vars := make([]interface{}, size*2)

	var pyKey, pyValue *C.PyObject
	var pos C.Py_ssize_t

	for i := 0; i < size; i++ {
		if C.PyDict_Next(kw, &pos, &pyKey, &pyValue) == 0 {
			logError("varsFromKw: can't get next item")
			if C.PyErr_Occurred() != nil {
				logError("error was: %s", pyLastError())
			}
			break
		}
		if C.py_type(pyKey) != C.PY_TYPE_UNICODE {
			logError("varsFromKw: key is not a string - %s", C.py_type_name(pyKey))
			continue
		}

		key := goString(pyKey)
		var value interface{}

		switch C.py_type(pyValue) {
		case C.PY_TYPE_UNICODE:
			value = goString(pyValue)
		case C.PY_TYPE_LONG:
			value = int(C.PyLong_AsLongLong(pyValue))
		case C.PY_TYPE_FLOAT:
			value = float64(C.PyFloat_AsDouble(pyValue))
		default:
			logError("varsFromKw: unsupported value type for %s - %s", key, C.py_type_name(pyValue))
			// TODO: Log
			//cName := C.py_type_name(value)
			continue
		}

		loc := i * 2
		vars[loc] = key
		vars[loc+1] = value
	}

	return vars
}

func eventPtr(event nuclio.Event) C.ulong {
	ptr := uintptr(unsafe.Pointer(&event))
	return C.ulong(ptr)
}

func loggerPtr(logger logger.Logger) C.ulong {
	ptr := uintptr(unsafe.Pointer(&logger))
	return C.ulong(ptr)
}

func eventFromPtr(ptr C.ulong) nuclio.Event {
	if ptr == 0 {
		panic("Event pointer is 0")
	}
	return *(*nuclio.Event)(unsafe.Pointer(uintptr(ptr)))
}

func loggerFromPtr(ptr C.ulong) logger.Logger {
	if ptr == 0 {
		panic("Logger pointer is 0")
	}
	return *(*logger.Logger)(unsafe.Pointer(uintptr(ptr)))
}

// nuclio.Event interface

// nolint
//export eventID
func eventID(ptr C.ulong) *C.PyObject {
	event := eventFromPtr(ptr)
	id := event.GetID().String()
	return pyString(id)
}

// nolint
//export eventTriggerInfo
func eventTriggerInfo(ptr C.ulong) *C.PyObject {
	event := eventFromPtr(ptr)
	triggerInfo := event.GetTriggerInfo()

	class := pyString(triggerInfo.GetClass())
	kind := pyString(triggerInfo.GetKind())

	return C.new_trigger_info(class, kind)
}

// nolint
//export eventContentType
func eventContentType(ptr C.ulong) *C.PyObject {
	event := eventFromPtr(ptr)
	ctype := event.GetContentType()
	return pyString(ctype)
}

// nolint
//export eventBody
func eventBody(ptr C.ulong) *C.PyObject {
	// TODO: https://docs.python.org/3.6/c-api/buffer.html to avoid allocations

	event := eventFromPtr(ptr)
	body := event.GetBody()
	return pyBytes(body)
}

// nolint
//export eventHeaders
func eventHeaders(ptr C.ulong) *C.PyObject {
	event := eventFromPtr(ptr)
	headers := event.GetHeaders()

	return pyDict(headers)
}

// nolint
//export eventFields
func eventFields(ptr C.ulong) *C.PyObject {
	event := eventFromPtr(ptr)
	fields := event.GetFields()

	return pyDict(fields)
}

// nolint
//export eventTimestamp
func eventTimestamp(ptr C.ulong) *C.PyObject {
	event := eventFromPtr(ptr)
	ts := event.GetTimestamp()
	return pyDateTime(ts)
}

// nolint
//export eventPath
func eventPath(ptr C.ulong) *C.PyObject {
	event := eventFromPtr(ptr)
	path := event.GetPath()
	return pyString(path)
}

// nolint
//export eventURL
func eventURL(ptr C.ulong) *C.PyObject {
	event := eventFromPtr(ptr)
	url := event.GetURL()
	return pyString(url)
}

// nolint
//export eventMethod
func eventMethod(ptr C.ulong) *C.PyObject {
	event := eventFromPtr(ptr)
	method := event.GetMethod()
	return pyString(method)
}

// nolint
//export eventShardID
func eventShardID(ptr C.ulong) *C.PyObject {
	event := eventFromPtr(ptr)
	shardID := event.GetShardID()
	return C.PyLong_FromLongLong(C.longlong(shardID))
}

// nolint
//export eventNumShards
func eventNumShards(ptr C.ulong) *C.PyObject {
	event := eventFromPtr(ptr)
	numShards := event.GetTotalNumShards()
	return C.PyLong_FromLongLong(C.longlong(numShards))
}

// nuclio.Logger interface

// nolint
//export loggerLog
func loggerLog(ptr C.ulong, level C.int, cMessage *C.char) {
	logger := loggerFromPtr(ptr)
	message := C.GoString(cMessage)

	switch level {
	case C.LOG_LEVEL_ERROR:
		logger.Error(message)
	case C.LOG_LEVEL_WARNING:
		logger.Warn(message)
	case C.LOG_LEVEL_INFO:
		logger.Info(message)
	case C.LOG_LEVEL_DEBUG:
		logger.Debug(message)
	default:
		logger.WarnWith("Unknown log level", "level", level)
		logger.Info(message)
	}
}

// nolint
//export loggerLogWith
func loggerLogWith(ptr C.ulong, level C.int, cMessage *C.char, kw *C.PyObject) {
	logger := loggerFromPtr(ptr)
	message := C.GoString(cMessage)
	vars := varsFromKw(kw)

	switch level {
	case C.LOG_LEVEL_ERROR:
		logger.ErrorWith(message, vars...)
	case C.LOG_LEVEL_WARNING:
		logger.WarnWith(message, vars...)
	case C.LOG_LEVEL_INFO:
		logger.InfoWith(message, vars...)
	case C.LOG_LEVEL_DEBUG:
		logger.DebugWith(message, vars...)
	default:
		logger.WarnWith("Unknown log level", "level", level)
		logger.Info(message)
	}
}
