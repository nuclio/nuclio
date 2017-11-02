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

# We can also configure the function inline - through a specially crafted
# comment such as the below. This is functionally equivalent to creating
# a function.yaml file.

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
