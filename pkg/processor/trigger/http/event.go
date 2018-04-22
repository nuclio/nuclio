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
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/valyala/fasthttp"
)

// allows accessing fasthttp.RequestCtx as a event.Sync
type Event struct {
	nuclio.AbstractEvent
	ctx *fasthttp.RequestCtx
}

// GetContentType returns the content type of the body
func (e *Event) GetContentType() string {
	return e.GetHeaderString("Content-Type")
}

// GetBody returns the body of the event
func (e *Event) GetBody() []byte {
	return e.ctx.Request.Body()
}

// GetHeaderByteSlice returns the header by name as a byte slice
func (e *Event) GetHeaderByteSlice(key string) []byte {

	// TODO: consider lifetime of the header. User may not keep a reference
	return e.ctx.Request.Header.Peek(key)
}

// GetHeader returns the header by name as an interface{}
func (e *Event) GetHeader(key string) interface{} {
	return e.GetHeaderByteSlice(key)
}

// GetHeaders loads all headers into a map of string / interface{}
func (e *Event) GetHeaders() map[string]interface{} {
	headers := make(map[string]interface{})
	e.ctx.Request.Header.VisitAll(func(key, value []byte) {
		headers[string(key)] = string(value)
	})

	return headers
}

// GetHeaderString returns the header by name as a string
func (e *Event) GetHeaderString(key string) string {
	return string(e.GetHeaderByteSlice(key))
}

// GetPath returns the method of the event, if applicable
func (e *Event) GetMethod() string {
	return string(e.ctx.Request.Header.Method())
}

// GetPath returns the path of the event
func (e *Event) GetPath() string {
	return string(e.ctx.Request.URI().Path())
}

// GetFieldByteSlice returns the field by name as a byte slice
func (e *Event) GetFieldByteSlice(key string) []byte {
	return e.ctx.QueryArgs().Peek(key)
}

// GetFieldString returns the field by name as a string
func (e *Event) GetFieldString(key string) string {
	return string(e.GetFieldByteSlice(key))
}

// GetFieldInt returns the field by name as an integer
func (e *Event) GetFieldInt(key string) (int, error) {
	return e.ctx.QueryArgs().GetUint(key)
}

// GetFields loads all fields into a map of string / interface{}
func (e *Event) GetFields() map[string]interface{} {
	fields := make(map[string]interface{})
	e.ctx.QueryArgs().VisitAll(func(key, value []byte) {
		fields[string(key)] = string(value)
	})

	return fields
}
