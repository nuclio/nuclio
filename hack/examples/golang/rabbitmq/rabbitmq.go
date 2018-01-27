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

//
// Listens to a RabbitMQ queue and records any messages posted to a given queue.
// Can retrieve these recorded messages through HTTP GET, demonstrating how a single
// function can be invoked from different triggers.
//

package main

import (
	"io/ioutil"
	"net/http"
	"os"

	"github.com/nuclio/nuclio-sdk-go"
)

const eventLogFilePath = "/tmp/events.json"

func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	context.Logger.InfoWith("Received event", "body", string(event.GetBody()))

	// if we got the event from rabbit
	if event.GetTriggerInfo().GetClass() == "async" && event.GetTriggerInfo().GetKind() == "rabbitMq" {

		eventLogFile, err := os.OpenFile(eventLogFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}

		defer eventLogFile.Close()

		// write the body followed by ', '
		for _, dataToWrite := range [][]byte{
			event.GetBody(),
			[]byte(", "),
		} {

			// write the thing to write
			if _, err = eventLogFile.Write(dataToWrite); err != nil {
				return nil, err
			}
		}

		// all's well
		return nil, nil
	}

	// open the log for read
	eventLogFile, err := os.OpenFile(eventLogFilePath, os.O_RDONLY, 0600)
	if err != nil {
		return nil, err
	}

	defer eventLogFile.Close()

	// read the entire file
	eventLogFileContents, err := ioutil.ReadAll(eventLogFile)
	if err != nil {
		return nil, err
	}

	// chop off the last 2 chars and enclose in a [ ]
	eventLogFileContentsString := "[" + string(eventLogFileContents[:len(eventLogFileContents)-2]) + "]"

	// return the contents as JSON
	return nuclio.Response{
		StatusCode:  http.StatusOK,
		ContentType: "application/json",
		Body:        []byte(eventLogFileContentsString),
	}, nil
}
