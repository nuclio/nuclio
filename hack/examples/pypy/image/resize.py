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

from PIL import Image


def handler(context, event):
    ratio = float(event.fields.get('ratio', '0.5'))

    img = Image.open(BytesIO(event.body))
    size = (int(ratio*img.size[0]), int(ratio*img.size[1]))
    rimg = img.resize(size)

    io = BytesIO()
    rimg.save(io, img.format)
    return io.getvalue()
