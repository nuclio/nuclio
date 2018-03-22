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

import "github.com/nuclio/nuclio-sdk-go"

func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
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
	"regexscan.go": `//
// Accept a string (event.body) and scan for compliance using a list of regex patterns (SSN, Credit Cards, ..)
// will return a list of compliance violations in Json or "Passed"
// demonstrate the use of structured and unstructured log with different levels
// can be extended to write results to a stream/object storage
//

package main

import (
	"encoding/json"
	"regexp"

	"github.com/nuclio/nuclio-sdk-go"
)

// list of regular expression filters
var rx = map[string]*regexp.Regexp{
	"SSN":         regexp.MustCompile("\\b\\d{3}-\\d{2}-\\d{4}\\b"),
	"Credit card": regexp.MustCompile("\\b(?:\\d[ -]*?){13,16}\\b")}

func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {

	// Unstructured debug message
	context.Logger.Debug("Process document %s, length %d", event.GetPath(), event.GetSize())

	data := string(event.GetBody())
	matchList := []string{}

	// Test content against a list of RegEx filters
	for k, v := range rx {
		if v.MatchString(string(data)) {
			matchList = append(matchList, "Contains "+k)
		}
	}

	// If we found a filter match add structured warning log message and respond with match list
	if len(matchList) > 0 {
		context.Logger.WarnWith("Document content warning", "path", event.GetPath(), "content", matchList)
		return json.Marshal(matchList)
	}

	return "Passed", nil
}`,
	"sentiments.py": `#
# uses vader lib (will be installed automatically via build commands) to identify sentiments in the body string
# return score result in the form of: {'neg': 0.0, 'neu': 0.323, 'pos': 0.677, 'compound': 0.6369}
#

from vaderSentiment.vaderSentiment import SentimentIntensityAnalyzer

def handler(context, event):
    body = event.body.decode('utf-8')
    context.logger.debug_with('Analyzing ', 'sentence', body)

    analyzer = SentimentIntensityAnalyzer()

    score = analyzer.polarity_scores(body)

    return str(score)
`,
	"tensor.py": `#
# This function uses TensorFlow to perform image recognition.
#
# You can try invoking this function by passing any .jpg image's URL
# in the request's body. For instance:
# http://www.lpcsathletics.org/wp-content/uploads/2017/08/soccer-5.jpg
#
# This example is based on the example in TensorFlow's official repository:
# https://github.com/tensorflow/models/blob/master/tutorials/image/imagenet/classify_image.py
#
# Simple image classification with Inception
#
# Runs image classification with Inception trained on ImageNet 2012
# Challenge data set.
#
# This program creates a graph from a saved GraphDef protocol buffer,
# and runs inference on an input JPEG image. It outputs human readable
# strings of up to the top 5 predictions along with their probabilities.
#

import os
import os.path
import re
import requests
import shutil
import tarfile
import traceback
import threading

import numpy as np
import tensorflow as tf


def handler(context, event):

    # we're going to need a unique temporary location to handle each event,
    # as we download a file as part of each function invocation
    temp_dir = Helpers.create_temporary_dir(context, event)

    # wrap everything with error handling such that any exception raised
    # at any point will still return a proper response
    try:

        # if we're not ready to handle this request yet, deny it
        if not FunctionState.done_loading:
            context.logger.warn_with('Model data not done loading yet, denying request')
            raise NuclioResponseError('Model data not loaded yet, cannot serve this request',
                                      requests.codes.service_unavailable)

        # read the event's body to determine the target image URL
        # TODO: in the future this can also take binary image data if provided with an appropriate content-type
        image_url = event.body.decode('utf-8').strip()

        # download the image to our temporary location
        image_target_path = os.path.join(temp_dir, 'downloaded_image.jpg')
        Helpers.download_file(context, image_url, image_target_path)

        # run the inference on the image
        results = Helpers.run_inference(context, image_target_path, 5, 0.3)

        # return a response with the result
        return context.Response(body=str(results),
                                headers={},
                                content_type='text/plain',
                                status_code=requests.codes.ok)

    # convert any NuclioResponseError to a response to be returned from our handler.
    # the response's description and status will appropriately convey the underlying error's nature
    except NuclioResponseError as error:
        return error.as_response(context)

    # if anything we didn't count on happens, respond with internal server error
    except Exception as error:
        context.logger.warn_with('Unexpected error occurred, responding with internal server error',
                                 exc=str(error))

        message = 'Unexpected error occurred: {0}\n{1}'.format(error, traceback.format_exc())
        return NuclioResponseError(message).as_response(context)

    # clean up after ourselves regardless of whether we succeeded or failed
    finally:
        shutil.rmtree(temp_dir)


class NuclioResponseError(Exception):

    def __init__(self, description, status_code=requests.codes.internal_server_error):
        self._description = description
        self._status_code = status_code

    def as_response(self, context):
        return context.Response(body=self._description,
                                headers={},
                                content_type='text/plain',
                                status_code=self._status_code)


class FunctionState(object):
    """
    This class has classvars that are set by methods invoked during file import,
    such that handler invocations can re-use them.
    """

    # holds the TensorFlow graph def
    graph = None

    # holds the node id to human string mapping
    node_lookup = None

    # holds a boolean indicating if we're ready to handle an invocation or haven't finished yet
    done_loading = False


class Paths(object):

    # the directory in the deployed function container where the data model is saved
    model_dir = os.getenv('MODEL_DIR', '/tmp/tfmodel/')

    # paths of files within the model archive used to create the graph
    label_lookup_path = os.path.join(model_dir,
                                     os.getenv('LABEL_LOOKUP_FILENAME',
                                               'imagenet_synset_to_human_label_map.txt'))

    uid_lookup_path = os.path.join(model_dir,
                                   os.getenv('UID_LOOKUP_FILENAME',
                                             'imagenet_2012_challenge_label_map_proto.pbtxt'))

    graph_def_path = os.path.join(model_dir,
                                  os.getenv('GRAPH_DEF_FILENAME',
                                            'classify_image_graph_def.pb'))


class Helpers(object):

    @staticmethod
    def create_temporary_dir(context, event):
        """
        Creates a uniquely-named temporary directory (based on the given event's id) and returns its path.
        """
        temp_dir = '/tmp/nuclio-event-{0}'.format(event.id)
        os.makedirs(temp_dir)

        context.logger.debug_with('Created temporary directory', path=temp_dir)

        return temp_dir

    @staticmethod
    def run_inference(context, image_path, num_predictions, confidence_threshold):
        """
        Runs inference on the image in the given path.
        Returns a list of up to N=num_prediction tuples (prediction human name, confidence score).
        Only takes predictions whose confidence score meets the provided confidence threshold.
        """

        # read the image binary data
        with tf.gfile.FastGFile(image_path, 'rb') as f:
            image_data = f.read()

        # run the graph's softmax tensor on the image data
        with tf.Session(graph=FunctionState.graph) as session:
            softmax_tensor = session.graph.get_tensor_by_name('softmax:0')
            predictions = session.run(softmax_tensor, {'DecodeJpeg/contents:0': image_data})
            predictions = np.squeeze(predictions)

        results = []

        # take the num_predictions highest scoring predictions
        top_predictions = reversed(predictions.argsort()[-num_predictions:])

        # look up each predicition's human-readable name and add it to the
        # results if it meets the confidence threshold
        for node_id in top_predictions:
            name = FunctionState.node_lookup[node_id]

            score = predictions[node_id]
            meets_threshold = score > confidence_threshold

            # tensorflow's float32 must be converted to float before logging, not JSON-serializable
            context.logger.info_with('Found prediction',
                                     name=name,
                                     score=float(score),
                                     meets_threshold=meets_threshold)

            if meets_threshold:
                results.append((name, score))

        return results

    @staticmethod
    def on_import():
        """
        This function is called when the file is imported, so that model data
        is loaded to memory only once per function deployment.
        """

        # load the graph def from trained model data
        FunctionState.graph = Helpers.load_graph_def()

        # load the node ID to human-readable string mapping
        FunctionState.node_lookup = Helpers.load_node_lookup()

        # signal that we're ready
        FunctionState.done_loading = True

    @staticmethod
    def load_graph_def():
        """
        Imports the GraphDef data into TensorFlow's default graph, and returns it.
        """

        # verify that the declared graph def file actually exists
        if not tf.gfile.Exists(Paths.graph_def_path):
            raise NuclioResponseError('Failed to find graph def file', requests.codes.service_unavailable)

        # load the TensorFlow GraphDef
        with tf.gfile.FastGFile(Paths.graph_def_path, 'rb') as f:
            graph_def = tf.GraphDef()
            graph_def.ParseFromString(f.read())

            tf.import_graph_def(graph_def, name='')

        return tf.get_default_graph()

    @staticmethod
    def load_node_lookup():
        """
        Composes the mapping between node IDs and human-readable strings. Returns the composed mapping.
        """

        # load the mappings from which we can build our mapping
        string_uid_to_labels = Helpers._load_label_lookup()
        node_id_to_string_uids = Helpers._load_uid_lookup()

        # compose the final mapping of integer node ID to human-readable string
        result = {}
        for node_id, string_uid in node_id_to_string_uids.items():
            label = string_uid_to_labels.get(string_uid)

            if label is None:
                raise NuclioResponseError('Failed to compose node lookup')

            result[node_id] = label

        return result

    @staticmethod
    def download_file(context, url, target_path):
        """
        Downloads the given remote URL to the specified path.
        """
        # make sure the target directory exists
        os.makedirs(os.path.dirname(target_path), exist_ok=True)
        try:
            with requests.get(url, stream=True) as response:
                response.raise_for_status()
                with open(target_path, 'wb') as f:
                    for chunk in response.iter_content(chunk_size=8192):
                        if chunk:
                            f.write(chunk)
        except Exception as error:
            if context is not None:
                context.logger.warn_with('Failed to download file',
                                         url=url,
                                         target_path=target_path,
                                         exc=str(error))
            raise NuclioResponseError('Failed to download file: {0}'.format(url),
                                      requests.codes.service_unavailable)
        if context is not None:
            context.logger.info_with('Downloaded file successfully',
                                     size_bytes=os.stat(target_path).st_size,
                                     target_path=target_path)

    @staticmethod
    def _load_label_lookup():
        """
        Loads and parses the mapping between string UIDs and human-readable strings. Returns the parsed mapping.
        """

        # verify that the declared label lookup file actually exists
        if not tf.gfile.Exists(Paths.label_lookup_path):
            raise NuclioResponseError('Failed to find Label lookup file', requests.codes.service_unavailable)

        # load the raw mapping data
        with tf.gfile.GFile(Paths.label_lookup_path) as f:
            lookup_lines = f.readlines()

        result = {}

        # parse the raw data to a mapping between string UIDs and labels
        # each line is expected to look like this:
        # n12557064     kidney bean, frijol, frijole
        line_pattern = re.compile(r'(n\d+)\s+([ \S,]+)')

        for line in lookup_lines:
            matches = line_pattern.findall(line)

            # extract the uid and label from the matches
            # in our example, uid will be "n12557064" and label will be "kidney bean, frijol, frijole"
            uid = matches[0][0]
            label = matches[0][1]

            # insert the UID and label to our mapping
            result[uid] = label

        return result

    @staticmethod
    def _load_uid_lookup():
        """
        Loads and parses the mapping between node IDs and string UIDs. Returns the parsed mapping.
        """

        # verify that the declared uid lookup file actually exists
        if not tf.gfile.Exists(Paths.uid_lookup_path):
            raise NuclioResponseError('Failed to find UID lookup file', requests.codes.service_unavailable)

        # load the raw mapping data
        with tf.gfile.GFile(Paths.uid_lookup_path) as f:
            lookup_lines = f.readlines()

        result = {}

        # parse the raw data to a mapping between integer node IDs and string UIDs
        # this file is expected to contains entries such as this:
        #
        # entry
        # {
        #   target_class: 443
        #   target_class_string: "n01491361"
        # }
        #
        # to parse it, we'll iterate over the lines, and for each line that begins with "  target_class:"
        # we'll assume that the next line has the corresponding "target_class_string"
        for i, line in enumerate(lookup_lines):

            # we found a line that starts a new entry in our mapping
            if line.startswith('  target_class:'):
                # target_class represents an integer value for node ID - convert it to an integer
                target_class = int(line.split(': ')[1])

                # take the string UID from the next line,
                # and clean up the quotes wrapping it (and the trailing newline)
                next_line = lookup_lines[i + 1]
                target_class_string = next_line.split(': ')[1].strip('"\n ')

                # insert the node ID and string UID to our mapping
                result[target_class] = target_class_string

        return result


# perform the loading in another thread to not block import - the function
# handler will gracefully decline requests until we're ready to handle them
t = threading.Thread(target=Helpers.on_import)
t.start()
`, "convert.sh": `#
# Installs ImageMagick and calls "convert" directly with the event body (e.g. an contents of an image) as stdin.
#
# If X-nuclio-arguments does not exist in the event headers, the default arguments passed to convert tells it to
# reduce the image to 50%. To run any other mode or any other setting, simply provide this header (note that this is
# unsanitized). For example, to reduce the received image to 10% of its size, set X-nuclio-arguments to
# "- -resize 10% fd:1"
#
# Invoke with httpie:
#
# http https://blog.golang.org/gopher/header.jpg | http localhost:<port> > thumb.jpg x-nuclio-arguments:"- -resize 20% fd:1"
`,
	"dates.js": `// Uses moment.js (installed as part of the build) to add a given amount of time
// to "now", and returns this as string. Invoke with a JSON containing:
//  - value: some number
//  - unit: some momentjs unit, as a string - e.g. days, d, hours, miliseconds
//
// For example, the following will add 3 hours to current time and return the response:
// {
//     "value": 3,
//     "unit": "hours"
// }
//

var moment = require('moment');

exports.handler = function(context, event) {
    var request = JSON.parse(event.body);
    var now = moment();

    context.logger.infoWith('Adding to now', {
        'request': request,
        'to': now.format()
    });

    now.add(request.value, request.unit);

    context.callback(now.format());
};
`,
	"SerializeObject.cs": `// Serialize an object and output the JSON result
// Sample function from https://www.newtonsoft.com/json/help/html/SerializingJSON.htm
using System;
using Newtonsoft.Json;
using Nuclio.Sdk;

public class nuclio
{
    public string SerializeObject(Context context, Event eventBase)
    {
        Product product = new Product();
        product.Name = System.Text.Encoding.UTF8.GetString(eventBase.Body);
        product.ExpiryDate = new DateTime(2008, 12, 28);
        product.Price = (double)3.99M;
        product.Sizes = new string[] { "Small", "Medium", "Large" };

        string output = JsonConvert.SerializeObject(product);
        //{
        //  "Name": "Apple", Product name
        //  "ExpiryDate": "2008-12-28T00:00:00",
        //  "Price": 3.99,
        //  "Sizes": [
        //    "Small",
        //    "Medium",
        //    "Large"
        //  ]
        //}

        return output;
    }

  public class Product {
      public string Name { get; set; }
      public DateTime ExpiryDate { get; set; }
      public double Price { get; set; }
      public string [] Sizes { get; set; }
  }
}
`,
	"ReverseEventHandler.java": `/* Simple Java handler that return the reverse of the event body */
import io.nuclio.Context;
import io.nuclio.Event;
import io.nuclio.EventHandler;
import io.nuclio.Response;

public class ReverseEventHandler implements EventHandler {
    
	@Override
    public Response handleEvent(Context context, Event event) {
       String body = new String(event.getBody());

       context.getLogger().infoWith("Got event", "body", body);
       String reversed = new StringBuilder(body).reverse().toString();

       return new Response().setBody(reversed);
    }
}
`,
	"s3watch.go": `// Watches and handles changes on S3 (via SNS) 
package main

import (
	"encoding/json"
	"net/http"

	"github.com/eawsy/aws-lambda-go-event/service/lambda/runtime/event/snsevt"
	"github.com/eawsy/aws-lambda-go-event/service/lambda/runtime/event/s3evt"
	"github.com/nuclio/nuclio-sdk-go"
)

// @nuclio.configure
// 
// function.yaml:
//   spec:
//     triggers:
//       myHttpTrigger:
//         maxWorkers: 4
//         kind: "http"
//         attributes:
//           ingresses:
//             http:
//               paths:
//               - "/mys3hook"

func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	context.Logger.DebugWith("Process document", "path", event.GetPath(), "body", string(event.GetBody()))

	// Get body, assume it is the right HTTP Post event, can add error checking
	body := event.GetBody()

	snsEvent := snsevt.Record{}
	err := json.Unmarshal([]byte(body),&snsEvent)
	if err != nil {
		return "", err
	}

	context.Logger.InfoWith("Got SNS Event", "type", snsEvent.Type)

	if snsEvent.Type == "SubscriptionConfirmation" {
		
		// need to confirm registration on first time
		context.Logger.DebugWith("Handle SubscriptionConfirmation",
			"TopicArn", snsEvent.TopicARN, 
			"Message", snsEvent.Message)

		resp, err := http.Get(snsEvent.SubscribeURL)
		if err != nil {
			context.Logger.ErrorWith("Failed to confirm SNS Subscription", "resp", resp, "err", err)
		}

		return "", nil
	}

	// Unmarshal S3 event, can add error check to verify snsEvent.TopicArn has the right topic (arn:aws:sns:...)
	myEvent := s3evt.Event{}
	json.Unmarshal([]byte(snsEvent.Message),&myEvent)

	context.Logger.InfoWith("Got S3 Event", "message", myEvent.String())

	// handle your S3 event here
	// ...

	return "", nil
}
`,
}
