/*
Copyright 2025 The Nuclio Authors.

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
	"strings"

	"github.com/nuclio/nuclio-sdk-go"
)

func Streamer(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	streamType := strings.ToLower(string(event.GetBody()))
	context.Logger.DebugWith("Got request", "path", event.GetPath(), "streamType", streamType)

	responseStream := nuclio.NewResponseStream(
		"text/plain",
		map[string]interface{}{
			"Content-Type": "text/plain",
		},
		200,
	)

	startStreamErr := make(chan error)

	go func() {
		defer responseStream.StopStreaming()

		switch streamType {
		case "alphabet":
			startStreamErr <- nil
			for ch := 'A'; ch <= 'Z'; ch++ {
				chunk := fmt.Sprintf("%c", ch)
				if _, err := responseStream.SendChunk([]byte(chunk)); err != nil {
					context.Logger.WarnWith("Failed to send chunk", "error", err)
					return
				}
			}
		case "numbers":
			startStreamErr <- nil
			for i := 0; i <= 10; i++ {
				chunk := fmt.Sprintf("%d", i)
				if _, err := responseStream.SendChunk([]byte(chunk)); err != nil {
					context.Logger.WarnWith("Failed to send chunk", "error", err)
					return
				}
			}
		case "text":
			startStreamErr <- nil
			lines := []string{
				"First sentence of the file.",
				"Second sentence of the file.",
				"Third sentence of the file.",
			}
			for _, line := range lines {
				if _, err := responseStream.SendChunk([]byte(line)); err != nil {
					context.Logger.WarnWith("Failed to send chunk", "error", err)
					return
				}
			}
		default:
			startStreamErr <- fmt.Errorf("Unsupported stream type: %s", streamType)
		}
	}()
	err := <-startStreamErr
	return responseStream, err
}
