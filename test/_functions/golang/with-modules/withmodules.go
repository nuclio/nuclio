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
	"github.com/nuclio/nuclio-sdk-go"

	"github.com/aidarkhanov/nanoid"
)

func WithModules(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	id := nanoid.New()
	context.Logger.DebugWith("Generated an id", "id", id)
	return nuclio.Response{
		StatusCode:  200,
		ContentType: "text/plain",
		Body:        []byte("from_go_modules"),
	}, nil
}
