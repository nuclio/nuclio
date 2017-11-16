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

import (
	"C"
	"encoding/json"
	"strings"
	"time"
	"unsafe"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/nuclio-sdk"
)

//export eventVersion
func eventVersion(ptr unsafe.Pointer) C.longlong {
	event := *(*nuclio.Event)(ptr)

	return C.longlong(event.GetVersion())
}

//export eventID
func eventID(ptr unsafe.Pointer) *C.char {
	event := *(*nuclio.Event)(ptr)

	return C.CString(event.GetID().String())
}

//export eventSize
func eventSize(ptr unsafe.Pointer) C.longlong {
	event := *(*nuclio.Event)(ptr)
	return C.longlong(event.GetSize())
}

//export eventTriggerClass
func eventTriggerClass(ptr unsafe.Pointer) *C.char {
	event := *(*nuclio.Event)(ptr)

	return C.CString(event.GetSource().GetClass())
}

//export eventTriggerKind
func eventTriggerKind(ptr unsafe.Pointer) *C.char {
	event := *(*nuclio.Event)(ptr)

	return C.CString(event.GetSource().GetKind())
}

//export eventContentType
func eventContentType(ptr unsafe.Pointer) *C.char {
	event := *(*nuclio.Event)(ptr)

	return C.CString(event.GetContentType())
}

//export eventBody
func eventBody(ptr unsafe.Pointer) *C.char {
	event := *(*nuclio.Event)(ptr)

	// TODO: Find how to pass byte array
	body := string(event.GetBody())
	return C.CString(body)
}

//export eventHeaderString
func eventHeaderString(ptr unsafe.Pointer, cKey *C.char) *C.char {
	event := *(*nuclio.Event)(ptr)
	key := C.GoString(cKey)

	value := event.GetHeaderString(key)
	return C.CString(value)
}

//export eventFieldString
func eventFieldString(ptr unsafe.Pointer, cKey *C.char) *C.char {
	event := *(*nuclio.Event)(ptr)
	key := C.GoString(cKey)

	value := event.GetFieldString(key)
	return C.CString(value)
}

//export eventTimestamp
func eventTimestamp(ptr unsafe.Pointer) C.double {
	event := *(*nuclio.Event)(ptr)

	timeStamp := event.GetTimestamp().UnixNano()
	epoch := float64(timeStamp) / float64(time.Second)

	return C.double(epoch)
}

//export eventPath
func eventPath(ptr unsafe.Pointer) *C.char {
	event := *(*nuclio.Event)(ptr)

	return C.CString(event.GetPath())
}

//export eventURL
func eventURL(ptr unsafe.Pointer) *C.char {
	event := *(*nuclio.Event)(ptr)

	return C.CString(event.GetURL())
}

//export eventMethod
func eventMethod(ptr unsafe.Pointer) *C.char {
	event := *(*nuclio.Event)(ptr)

	return C.CString(event.GetMethod())
}

/*
Event TODO:

GetHeader(key string) interface{}
GetHeaderByteSlice(key string) []byte
GetHeaders() map[string]interface{}
GetField(key string) interface{}
GetFieldByteSlice(key string) []byte
GetFieldInt(key string) (int, error)
GetFields() map[string]interface{}
*/

//export contextLogError
func contextLogError(ptr unsafe.Pointer, cMessage *C.char) {
	context := (*nuclio.Context)(ptr)
	message := C.GoString(cMessage)

	context.Logger.Error(message)
}

//export contextLogWarn
func contextLogWarn(ptr unsafe.Pointer, cMessage *C.char) {
	context := (*nuclio.Context)(ptr)
	message := C.GoString(cMessage)

	context.Logger.Warn(message)
}

//export contextLogInfo
func contextLogInfo(ptr unsafe.Pointer, cMessage *C.char) {
	context := (*nuclio.Context)(ptr)
	message := C.GoString(cMessage)

	context.Logger.Info(message)
}

//export contextLogDebug
func contextLogDebug(ptr unsafe.Pointer, cMessage *C.char) {
	context := (*nuclio.Context)(ptr)
	message := C.GoString(cMessage)

	context.Logger.Debug(message)
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

//export contextLogInfoWith
func contextLogInfoWith(ptr unsafe.Pointer, cFormat *C.char, cVars *C.char) {
	context := (*nuclio.Context)(ptr)
	format := C.GoString(cFormat)
	varsJSON := C.GoString(cVars)

	vars, err := parseVars(varsJSON)
	if err != nil {
		context.Logger.WarnWith("Can't parse vars JSON", "error", err, "vars", varsJSON)
		vars = []interface{}{"vars", varsJSON}
	}

	context.Logger.InfoWith(format, vars...)
}

/*
// flushes buffered logs, if applicable
Flush()

// returns a child logger, if underlying logger supports hierarchal logging
GetChild(name string) Logger
*/
