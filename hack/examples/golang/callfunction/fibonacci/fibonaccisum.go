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
	"strconv"

	"github.com/nuclio/nuclio-sdk-go"
)

func newFibFunction(ctx *nuclio.Context) func(uint64) (uint64, error) {
	return func(num uint64) (uint64, error) {
		body := strconv.FormatUint(num, 10)
		event := &nuclio.MemoryEvent{Body: []byte(body)}
		response, err := ctx.Platform.CallFunction("fibonacci", event)
		if err != nil {
			return 0, err
		}

		result, err := strconv.ParseUint(string(response.Body), 10, 0)
		if err != nil {
			return 0, err
		}
		return result, nil
	}
}

func fibSum(fib func(uint64) (uint64, error), num ...uint64) (uint64, error) {
	var result uint64
	for _, v := range num {
		value, err := fib(v)
		if err != nil {
			return 0, err
		}
		result += value
	}
	return result, nil
}

func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	result, err := fibSum(newFibFunction(context), 2, 10, 17) // => 1653
	if err != nil {
		return nil, err
	}
	return nuclio.Response{
		StatusCode:  200,
		ContentType: "application/text",
		Body:        []byte(strconv.FormatUint(result, 10)),
	}, nil
}
