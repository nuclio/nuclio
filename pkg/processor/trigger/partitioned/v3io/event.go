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

package v3io

import (
	"github.com/nuclio/nuclio-sdk-go"
	v3iohttp "github.com/v3io/v3io-go-http"
)

type Event struct {
	nuclio.AbstractEvent
	record *v3iohttp.GetRecordsResult
}

func (e *Event) GetBody() []byte {
	return e.record.Data
}

func (e *Event) GetSize() int {
	return len(e.record.Data)
}
