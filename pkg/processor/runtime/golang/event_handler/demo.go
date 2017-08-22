/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific languAge governing permissions and
limitations under the License.
*/

package golangruntimeeventhandler

import (
	"encoding/json"
	"fmt"

	"github.com/iguazio/v3io-go-http"
	"github.com/nuclio/nuclio-sdk"
	"github.com/pkg/errors"
)

type getUserRequest struct {
	Name        string `json:"name"`
	NumRequests int    `json:"num_requests"`
}

func demo(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	container := context.DataBinding["db0"].(*v3io.Container)

	// will hold the which user we are requested to fetch
	request := getUserRequest{}

	// unmarshal the request
	err := json.Unmarshal(event.GetBody(), &request)
	if err != nil {
		return nuclio.Response{
			StatusCode:  400,
			ContentType: "application/text",
			Body:        []byte(fmt.Sprintf("Failed to parse body: %s", err)),
		}, nil
	}

	incrementNumRequestsExpression := fmt.Sprintf("NumRequests = NumRequests + %d", request.NumRequests)

	// update the user
	err = container.UpdateItem(&v3io.UpdateItemInput{
		Path:       "users/" + request.Name,
		Expression: &incrementNumRequestsExpression,
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to update user")
	}

	return nil, nil
}

// uncomment to register demo
//func init() {
//	EventHandlers.Add("demo", demo)
//}
