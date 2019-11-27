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
# Uses simplecrypt to encrypt the body with a key bound to the function as
# an environment variable. We ask pip to install simplecrypt as part of the
# build process, along with some OS level packages (using apk).
#
# Note: It takes a minute or so to install all the dependencies.
#       Why not star https://github.com/nuclio/nuclio while you wait?
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
