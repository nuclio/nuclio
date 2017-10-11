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
}
