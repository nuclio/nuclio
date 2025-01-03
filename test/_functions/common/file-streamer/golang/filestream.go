/*
Copyright 2023 The Nuclio Authors.

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
	nuclio "github.com/nuclio/nuclio-sdk-go"
)

func FileStreamer(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	context.Logger.DebugWith("Got request", "path", event.GetPath())

	headers := map[string]interface{}{
		"X-nuclio-filestream-path": event.GetPath(),
		"X-request-body":           string(event.GetBody()),
	}

	if event.GetFieldString("delete_after_send") == "true" {
		headers["X-nuclio-filestream-delete-after-send"] = "true"
	}

	return nuclio.Response{
		Headers: headers,
	}, nil
}
