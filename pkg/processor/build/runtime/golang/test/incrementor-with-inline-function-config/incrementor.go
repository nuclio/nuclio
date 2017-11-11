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

// @nuclio.configure
//
// function.yaml:
//   spec:
//     triggers:
//
//       incrementor_http:
//         maxWorkers: 4
//         kind: "http"
//
//       rmq:
//         kind: "rabbit-mq"
//         url: "amqp://guest:guest@34.224.60.166:5672"
//         attributes:
//           exchangeName: "functions"
//           queueName: "functions"
//
//     dataBindings:
//       db0:
//         class: "v3io"
//         secret: "something"
//         url: "http://192.168.51.240:8081/1024"
//

package main

import (
	"github.com/nuclio/nuclio-sdk"
)

func Increment(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	incrementedBody := []byte{}

	context.Logger.InfoWith("Incrementing body", "body", string(event.GetBody()))

	for _, byteValue := range event.GetBody() {
		incrementedBody = append(incrementedBody, byteValue+1)
	}

	return incrementedBody, nil
}
