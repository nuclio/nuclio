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
	"time"
	"unsafe"

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

//export eventHeader
func eventHeader(ptr unsafe.Pointer, cKey *C.char) *C.char {
	event := *(*nuclio.Event)(ptr)
	key := C.GoString(cKey)

	return C.CString(event.GetHeaderString(key))
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
TODO:

GetHeader(key string) interface{}
GetHeaderByteSlice(key string) []byte
GetHeaders() map[string]interface{}
GetField(key string) interface{}
GetFieldByteSlice(key string) []byte
GetFieldString(key string) string
GetFieldInt(key string) (int, error)
GetFields() map[string]interface{}
*/
