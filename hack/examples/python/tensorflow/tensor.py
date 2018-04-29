# Copyright 2017 The Nuclio Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# This function uses TensorFlow to perform image recognition.
# It takes advantage of nuclio's inline configuration to indicate
# its pip dependencies, as well as the linux distribution
# to use for the deployed function's container.
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


def classify(context, event):

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
