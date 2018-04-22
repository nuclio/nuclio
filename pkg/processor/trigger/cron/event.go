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

package cron

import (
	"fmt"

	"github.com/nuclio/nuclio-sdk-go"
)

type Event struct {
	nuclio.AbstractEvent
	Body    string
	Headers map[string]interface{}
}

func (e *Event) GetBody() []byte {
	return []byte(e.Body)
}

func (e *Event) GetHeader(key string) interface{} {
	return e.Headers[key]
}

func (e *Event) GetHeaderByteSlice(key string) []byte {
	return []byte(e.GetHeaderString(key))
}

func (e *Event) GetHeaderString(key string) string {
	if headerValue, headerExists := e.Headers[key]; headerExists {
		return fmt.Sprintf("%v", headerValue)
	}

	return ""
}

func (e *Event) GetHeaders() map[string]interface{} {
	return e.Headers
}
