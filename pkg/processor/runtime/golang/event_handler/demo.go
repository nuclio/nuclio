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

package golangruntimeeventhandler

import (
	"github.com/nuclio/nuclio-sdk"

	"github.com/iguazio/v3io-go-http"
	"github.com/nuclio/nuclio/pkg/v3ioclient"
)

func demo(context *nuclio.Context, event nuclio.Event) (interface{}, error) {

	container := context.DataBinding["db0"].(*v3ioclient.V3ioClient)
	err := container.PutObject(&v3io.PutObjectInput{
		"foo.txt",
		[]byte("This is the contents"),
	})

	if err != nil {
		return nil, err
	}

	response, err := container.GetObject(&v3io.GetObjectInput{
		"foo.txt",
	})

	if err != nil {
		return nil, err
	}

	// release response we got
	response.Release()

	return string(response.Body()), nil
}

// uncomment to register demo
func init() {
	EventHandlers.Add("demo", demo)
}
