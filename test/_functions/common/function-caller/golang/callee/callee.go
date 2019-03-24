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
	"errors"

	"github.com/nuclio/nuclio-sdk-go"
)

func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	var parsedEventBody map[string]string
	if err := json.Unmarshal(event.GetBody(), &parsedEventBody); err != nil {
		return nil, err
	}

	returnValue, returnValueFound := parsedEventBody["return_this"]
	if !returnValueFound {
		return nil, errors.New("input event don't have callee_name")
	}

	data, err := json.Marshal(map[string]string{"from_callee": returnValue})
	if err != nil {
		return nil, err
	}

	headers := map[string]interface{}{
		"X-Callee-Received-Header": "caller_header",
	}

	return nuclio.Response{
		StatusCode:  201,
		ContentType: "application/text",
		Body:        data,
		Headers:     headers,
	}, nil
}
