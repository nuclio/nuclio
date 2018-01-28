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
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/nuclio/nuclio-sdk-go"
)

func Outputter(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	if event.GetMethod() != "POST" {
		return event.GetMethod(), nil
	}

	body := string(event.GetBody())

	switch body {
	case "return_string":
		return "a string", nil

	case "return_bytes":
		return []byte{'b', 'y', 't', 'e', 's'}, nil

	case "log":
		context.Logger.Debug("Debug message")
		context.Logger.Info("Info message")
		context.Logger.Warn("Warn message")
		context.Logger.Error("Error message")

		return "returned logs", nil

	case "return_response":
		headers := map[string]interface{}{}
		headers["a"] = event.GetHeaderString("a")
		headers["b"] = event.GetHeaderString("b")
		headers["h1"] = "v1"
		headers["h2"] = "v2"

		return nuclio.Response{
			StatusCode:  http.StatusCreated,
			ContentType: "text/plain; charset=utf-8",
			Headers:     headers,
			Body:        []byte("response body"),
		}, nil

	case "panic":
		panic("Panicking, as per request")

	case "return_fields":
		var fields []string
		for field, value := range event.GetFields() {
			fields = append(fields, fmt.Sprintf("%s=%v", field, value))
		}
		// We use sorted to get predictable output
		sort.Strings(fields)
		return strings.Join(fields, ","), nil

	case "return_path":
		return event.GetPath(), nil
	}

	return nil, nuclio.ErrInternalServerError
}
