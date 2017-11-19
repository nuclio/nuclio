package fixtures

// Sources contains a map of built in source fixtures
var Sources = map[string]string{
	"echo.go": `//
// Super simple Golang function that echoes back the body it receives
//
// Note: The first build takes longer as it performs one time initializations (e.g.
// pulls golang:1.8-alpine3.6 from docker hub).
//

package main

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
# function.yaml:
#   apiVersion: "nuclio.io/v1"
#   kind: "Function"
#   spec:
#     runtime: "python"
#     handler: "encrypt:encrypt"
#
#     build:
#       commands:
#       - "apk update"
#       - "apk add --no-cache gcc g++ make libffi-dev openssl-dev"
#       - "pip install simple-crypt"
#

import os
import simplecrypt

def encrypt(context, event):
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
// Can retrieve these recorded messages through HTTP GET, demonstrating how a single
// function can be invoked from different triggers.
//

// @nuclio.configure
//
// function.yaml:
//   apiVersion: "nuclio.io/v1"
//   kind: "Function"
//   spec:
//
//     # Note that we're not specifying handler. This is because the Golang runtimg
//     # can find handlers automatically.
//     runtime: "golang"
//
//     # you'll need to specify the user, password and address of a working rabbit mq
//     # to actually see this working
//     triggers:
//       test_rmq:
//         kind: "rabbit-mq"
//         url: "amqp://user:password@rabbitmq-host:5672"
//         attributes:
//           exchangeName: "exchange-name"
//           queueName: "queue-name"
//

package main

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
	"face.py": `#
# Uses Microsoft's Face API to extract face information from the
# picture whose URL is submitted in the request body. The result is
# returned as a table of face objects sorted by their center's
# position in the given picture, left-to-right and then top-to-bottom.
#
# You will need a valid key from Microsoft:
# https://azure.microsoft.com/en-gb/try/cognitive-services/?api=face-api
#
# Once a valid Face API key has been acquired, set it and the appropriate
# regional base URL as the environment for this function
# (in the config section).
#

# @nuclio.configure
#
# function.yaml:
#   apiVersion: "nuclio.io/v1"
#   kind: "Function"
#   spec:
#     build:
#       commands:
#       - "pip install cognitive_face tabulate inflection"
#

import os
import cognitive_face as cf
import tabulate
import inflection


def handler(context, event):

    # extract the stuff we need
    image_url = event.body.decode('utf-8').strip()
    key = os.environ.get('FACE_API_KEY')
    base_url = os.environ.get('FACE_API_BASE_URL')

    if key is None:
        context.logger.warn('Face API key not set, cannot continue')
        return _build_response(context, 'Function misconfigured: Face API key not set', 503)

    if base_url is None:
        context.logger.warn('Face API base URL not set, cannot continue')
        return _build_response(context, 'Function misconfigured: Face API base URL not set', 503)

    if not image_url:
        context.logger.warn('No URL given in request body')
        return _build_response(context, 'Image URL required', 400)

    # configure cognitive face wrapper
    cf.Key.set(key)
    cf.BaseUrl.set(base_url)

    # attempt to request using the provided info
    try:
        context.logger.info('Requesting detection from Face API: {0}'.format(image_url))
        detected_faces = cf.face.detect(image_url,
                                        face_id=False,
                                        attributes='age,gender,glasses,smile,emotion')
    except Exception as error:
        context.logger.warn('Face API error occurred: {0}'.format(error))
        return _build_response(context, 'Face API error occurred', 503)

    parsed_faces = []

    # determine the center point of each detected face and map it to its attributes,
    # as well as clean up the retreived data for viewing comfort
    for face in detected_faces:
        coordinates = face['faceRectangle']
        attributes = face['faceAttributes']

        center_x = coordinates['left'] + coordinates['width'] / 2
        center_y = coordinates['top'] + coordinates['height'] / 2

        # determine the primary emotion based on its weighing
        primary_emotion = sorted(attributes['emotion'].items(), key=lambda item: item[1])[-1][0]

        parsed_face = {
            'x': center_x,
            'y': center_y,
            'position': '({0},{1})'.format(int(center_x), int(center_y)),
            'gender': inflection.humanize(attributes['gender']),
            'age': int(attributes['age']),
            'glasses': inflection.humanize(inflection.underscore(attributes['glasses'])),
            'primary_emotion': inflection.humanize(primary_emotion),
            'smile': '{0:.1f}%'.format(attributes['smile'] * 100),
        }

        parsed_faces.append(parsed_face)

    # sort according to center point, first x then y
    parsed_faces.sort(key=lambda face: (face['x'], face['y']))

    # prepare the data for tabulation
    first_row = ('',) + tuple(face['position'] for face in parsed_faces)
    make_row = lambda name: (inflection.humanize(name),) + tuple(
                            face[name] for face in parsed_faces)

    other_rows = [make_row(name) for name in [
                  'gender', 'age', 'primary_emotion', 'glasses', 'smile']]

    # return the human-readable face data in a neat table format
    return _build_response(context,
                           tabulate.tabulate([first_row] + other_rows,
                                             headers='firstrow',
                                             tablefmt='fancy_grid',
                                             numalign='center',
                                             stralign='center'),
                           200)


def _build_response(context, body, status_code):
    return context.Response(body=body,
                            headers={},
                            content_type='text/plain',
                            status_code=status_code)
`,
}
