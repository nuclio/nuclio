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

package http

import (
	"io/ioutil"
	net_http "net/http"
	"strconv"
	"time"

	"github.com/nuclio/nuclio-sdk-go"
)

type NetEvent struct {
	nuclio.AbstractEvent
	request        *net_http.Request
	responseWriter net_http.ResponseWriter
	body           *[]byte
}

// GetContentType returns the content type of the body
func (e *NetEvent) GetContentType() string {
	return e.request.Header.Get("Content-Type")
}

// GetBody returns the body of the event
func (e *NetEvent) GetBody() []byte {
	if e.body != nil {
		return *e.body
	}

	body, _ := ioutil.ReadAll(e.request.Body)

	// the body can only be read once. so read it once and store it in the event
	e.body = &body

	return body
}

// GetHeaderByteSlice returns the header by name as a byte slice
func (e *NetEvent) GetHeaderByteSlice(key string) []byte {
	return []byte(e.request.Header.Get(key))
}

// GetHeader returns the header by name as an interface{}
func (e *NetEvent) GetHeader(key string) interface{} {
	return e.request.Header.Get(key)
}

// GetHeaders loads all headers into a map of string / interface{}
func (e *NetEvent) GetHeaders() map[string]interface{} {
	headers := map[string]interface{}{}

	for headerKey, headerValue := range e.request.Header {
		headers[headerKey] = headerValue[0]
	}

	return headers
}

// GetHeaderString returns the header by name as a string
func (e *NetEvent) GetHeaderString(key string) string {
	return string(e.GetHeaderByteSlice(key))
}

// GetPath returns the method of the event, if applicable
func (e *NetEvent) GetMethod() string {
	return e.request.Method
}

// GetPath returns the path of the event
func (e *NetEvent) GetPath() string {
	return e.request.URL.Path
}

// GetFieldByteSlice returns the field by name as a byte slice
func (e *NetEvent) GetFieldByteSlice(key string) []byte {
	return []byte(e.request.URL.Query().Get(key))
}

// GetFieldString returns the field by name as a string
func (e *NetEvent) GetFieldString(key string) string {
	return string(e.GetFieldByteSlice(key))
}

// GetFieldInt returns the field by name as an integer
func (e *NetEvent) GetFieldInt(key string) (int, error) {
	return strconv.Atoi(e.GetFieldString(key))
}

// GetFields loads all fields into a map of string / interface{}
func (e *NetEvent) GetFields() map[string]interface{} {
	fields := make(map[string]interface{})
	for fieldKey, fieldValue := range e.request.URL.Query() {
		fields[fieldKey] = fieldValue[0]
	}

	return fields
}

// GetTimestamp returns when the event originated
func (e *NetEvent) GetTimestamp() time.Time {
	return time.Now()
}
