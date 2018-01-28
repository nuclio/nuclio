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

package v3ioitempoller

import (
	"time"

	"github.com/iguazio/v3io"
	"github.com/nuclio/nuclio-sdk-go"
)

type Event struct {
	nuclio.AbstractEvent
	item *v3io.ItemRespStruct
	url  string
	path string
}

func (e *Event) GetHeader(key string) interface{} {
	return (*e.item)[key]
}

func (e *Event) GetHeaders() map[string]interface{} {
	return *e.item
}

func (e *Event) GetTimestamp() time.Time {
	secs := e.GetHeader("__mtime_secs").(int)
	nsecs := e.GetHeader("__mtime_nsecs").(int)

	return time.Unix(int64(secs), int64(nsecs))
}

func (e *Event) GetSize() int {
	return e.GetHeader("__size").(int)
}

func (e *Event) GetURL() string {
	return e.url
}

func (e *Event) GetPath() string {
	return e.path
}
