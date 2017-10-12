package fixtures

// Sources contains a map of built in source fixtures
var Sources = map[string]string{
	"echo.go": `//
// Super simple Golang function that echoes back the body it receives
//
// Note: The first build takes longer as it performs one time initializations (e.g.
// pulls golang:1.8-alpine3.6 from docker hub).
//

package echo

import "github.com/nuclio/nuclio-sdk"

func Echo(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	return event.GetBody(), nil
}
`,
	"encrypt.py": `#
# Uses simplecrypt to encrypt the body with a key bound to the function as
# an environment variable. We ask pip to install simplecrypt as part of the
# build process, along with some OS level packages (using apk).
#
# Note: It takes a minute or so to install all the dependencies.
#       Why not star https://github.com/nuclio/nuclio while you wait?
#

# @nuclio.configure
#
# build.yaml:
#   commands:
#     - apk update
#     - apk add --no-cache gcc g++ make libffi-dev openssl-dev
#     - pip install simple-crypt
#

import os
import simplecrypt

def handler(context, event):
	context.logger.info('Using secret to encrypt body')

	# get the encryption key
	encryption_key = os.environ.get('ENCRYPT_KEY', 'some-default-key')

	# encrypt the body
	encrypted_body = simplecrypt.encrypt(encryption_key, event.body)

	# return the encrypted body, and some hard-coded header
	return context.Response(body=str(encrypted_body),
							headers={'x-encrypt-algo': 'aes256'},
							content_type='text/plain',
							status_code=200)
`,
	"rabbitmq.go": `//
// Listens to a RabbitMQ queue and records any messages posted to a given queue.
// Can retreive these recorded messages through HTTP GET, demonstrating how a single
// function can be invoked from different event sources.
//

// @nuclio.configure
//
// processor.yaml:
//   event_sources:
//     test_rmq:
//       class: "async"
//       kind: "rabbit-mq"
//       enabled: true
//       url: "amqp://<user>:<password>@<rabbitmq-host>:5672"
//       exchange: "<exchange name>"
//       queue_name: "<queue name">
//

package eventrecorder

import (
	"io/ioutil"
	"net/http"
	"os"

	"github.com/nuclio/nuclio-sdk"
)

const eventLogFilePath = "/tmp/events.json"

func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	context.Logger.InfoWith("Received event", "body", string(event.GetBody()))

	// if we got the event from rabbit
	if event.GetSource().GetClass() == "async" && event.GetSource().GetKind() == "rabbitMq" {

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
`,
}
