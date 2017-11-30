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

// TODO: Can we somehow unite with pypy/interface.go?

package nodejs

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
	"unsafe"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
)

/*
#include "log_levels.h"
*/
import "C"

var (
	logger nuclio.Logger
)

func logError(message string, args ...interface{}) {
	if logger == nil {
		var err error
		logger, err = nucliozap.NewNuclioZapCmd("pypy", nucliozap.ErrorLevel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Can't create logger - %s\n", err)
			fmt.Fprintf(os.Stderr, "\tMESSAGE: %s\n", message)
			fmt.Fprintf(os.Stderr, "\tARGS: %v\n", args)
			return
		}
	}

	logger.ErrorWith(message, args)
}

// nolint
//export eventVersion
func eventVersion(ptr unsafe.Pointer) C.longlong {
	event := *(*nuclio.Event)(ptr)

	return C.longlong(event.GetVersion())
}

// nolint
//export eventID
func eventID(ptr unsafe.Pointer) *C.char {
	event := *(*nuclio.Event)(ptr)

	return C.CString(event.GetID().String())
}

// nolint
//export eventSize
func eventSize(ptr unsafe.Pointer) C.longlong {
	event := *(*nuclio.Event)(ptr)
	return C.longlong(event.GetSize())
}

// nolint
//export eventTriggerClass
func eventTriggerClass(ptr unsafe.Pointer) *C.char {
	event := *(*nuclio.Event)(ptr)

	return C.CString(event.GetSource().GetClass())
}

// nolint
//export eventTriggerKind
func eventTriggerKind(ptr unsafe.Pointer) *C.char {
	event := *(*nuclio.Event)(ptr)

	return C.CString(event.GetSource().GetKind())
}

// nolint
//export eventContentType
func eventContentType(ptr unsafe.Pointer) *C.char {
	event := *(*nuclio.Event)(ptr)

	return C.CString(event.GetContentType())
}

// nolint
//export eventBody
func eventBody(ptr unsafe.Pointer) *C.char {
	event := *(*nuclio.Event)(ptr)

	body := string(event.GetBody())
	return C.CString(body)
}

// nolint
//export eventHeaders
func eventHeaders(ptr unsafe.Pointer) *C.char {
	event := *(*nuclio.Event)(ptr)

	headers := event.GetHeaders()
	data, err := json.Marshal(headers)
	if err != nil {
		logError("Can't marshal headers", "headers", headers)
		data = []byte("{}")
	}

	return C.CString(string(data))
}

// nolint
//export eventFields
func eventFields(ptr unsafe.Pointer) *C.char {
	event := *(*nuclio.Event)(ptr)

	fields := event.GetFields()
	data, err := json.Marshal(fields)
	if err != nil {
		logError("Can't marshal fields", "fields", fields)
		data = []byte("{}")
	}

	return C.CString(string(data))
}

// nolint
//export eventTimestamp
func eventTimestamp(ptr unsafe.Pointer) C.double {
	event := *(*nuclio.Event)(ptr)

	timeStamp := event.GetTimestamp().UnixNano()
	epoch := float64(timeStamp) / float64(time.Millisecond)

	return C.double(epoch)
}

// nolint
//export eventPath
func eventPath(ptr unsafe.Pointer) *C.char {
	event := *(*nuclio.Event)(ptr)

	return C.CString(event.GetPath())
}

// nolint
//export eventURL
func eventURL(ptr unsafe.Pointer) *C.char {
	event := *(*nuclio.Event)(ptr)

	return C.CString(event.GetURL())
}

// nolint
//export eventMethod
func eventMethod(ptr unsafe.Pointer) *C.char {
	event := *(*nuclio.Event)(ptr)

	return C.CString(event.GetMethod())
}

// nolint
//export contextLog
func contextLog(ptr unsafe.Pointer, level C.int, cMessage *C.char) {
	context := (*nuclio.Context)(ptr)
	message := C.GoString(cMessage)

	switch level {
	case C.LOG_LEVEL_ERROR:
		context.Logger.Error(message)
	case C.LOG_LEVEL_WARNING:
		context.Logger.Warn(message)
	case C.LOG_LEVEL_INFO:
		context.Logger.Info(message)
	case C.LOG_LEVEL_DEBUG:
		context.Logger.Debug(message)
	default:
		context.Logger.WarnWith("Unknown log level", "level", level)
		context.Logger.Info(message)
	}
}

// parseVars parses vars encoded as JSON object
func parseVars(varsJSON string) ([]interface{}, error) {
	var vars map[string]interface{}

	dec := json.NewDecoder(strings.NewReader(varsJSON))
	if err := dec.Decode(&vars); err != nil {
		return nil, err
	}

	return common.MapToSlice(vars), nil
}

// nolint
//export contextLogWith
func contextLogWith(ptr unsafe.Pointer, level C.int, cFormat *C.char, cVars *C.char) {
	context := (*nuclio.Context)(ptr)
	format := C.GoString(cFormat)
	varsJSON := C.GoString(cVars)

	vars, err := parseVars(varsJSON)
	if err != nil {
		context.Logger.WarnWith("Can't parse vars JSON", "error", err, "vars", varsJSON)
		vars = []interface{}{"vars", varsJSON}
	}

	switch level {
	case C.LOG_LEVEL_ERROR:
		context.Logger.ErrorWith(format, vars...)
	case C.LOG_LEVEL_WARNING:
		context.Logger.WarnWith(format, vars...)
	case C.LOG_LEVEL_INFO:
		context.Logger.InfoWith(format, vars...)
	case C.LOG_LEVEL_DEBUG:
		context.Logger.DebugWith(format, vars...)
	default:
		context.Logger.WarnWith("Unknown log level", "level", level)
		context.Logger.InfoWith(format, vars...)
	}
}
