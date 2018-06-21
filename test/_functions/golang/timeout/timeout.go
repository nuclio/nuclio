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

package main

import (
	"encoding/json"
	"os"
	"time"

	"github.com/nuclio/nuclio-sdk-go"
)

type timeoutRequest struct {
	Timeout string `json:"timeout"`
}

// Handler is timeout handler
func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {

	var request timeoutRequest
	if err := json.Unmarshal(event.GetBody(), &request); err != nil {
		return nil, err
	}

	timeout, err := time.ParseDuration(request.Timeout)
	if err != nil {
		return nil, err
	}

	context.Logger.InfoWith("Sleeping", "timeout", timeout.String())
	time.Sleep(timeout)
	responseMessage := map[string]interface{}{
		"pid": os.Getpid(),
	}

	response := nuclio.Response{
		ContentType: "application/json",
	}

	response.Body, err = json.Marshal(responseMessage)
	if err != nil {
		return nil, err
	}

	return response, nil
}
