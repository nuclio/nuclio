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
# Uses pillow to resize images. Ration is provided as URL parameter
# Example call:
#   curl -o out.png --data-binary @image.png http://localhost:8080?ratio=0.1
#
# We ask pip to install pillow as part of the build process.
#
# Note: It takes a minute or so to install all the dependencies.
#       Why not star https://github.com/nuclio/nuclio while you wait?

from io import BytesIO
from urllib2 import urlopen

from PIL import Image


def handler(context, event):
    x = int(event.fields.get('x', '100'))
    y = int(event.fields.get('y', '100'))
    format = event.fields.get('format', 'png')
    ctype = event.content_type
    data = event.body

    context.logger.debug_with(
        'Got request', path=event.path, x=x, y=y, format=format, ctype=ctype)

    # Assume it's URL to image if we got plain text
    if ctype.startswith('text/plain'):
        url = data.decode('utf-8')
        context.logger.debug_with('Getting image', url=url)
        data = urlopen(url).read()

    img = Image.open(BytesIO(data))
    img.thumbnail([x, y])

    io = BytesIO()
    img.save(io, format)

    return context.Response(
        body=io.getvalue(),
        content_type='image/{}'.format(format),
    )
