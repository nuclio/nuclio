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

/*
Package nodejs implements nodejs runtime

nodejs code need to export a handler function:

	exports.handler = function(context, event) {
		// ...
		context.callback(reply); // MUST return reply via callback
	}

context is current call context, it contains the following:
    callback:
		Callback function, *must* be used to return response.
		Response can be one of:
			- string
			- Buffer
			- array of [status, body]
			- context.Response object

		Response:
			A response. Has the following fields
				- body
				- headers
				- content_type
				- status_code

		Logging functions:
			- logger.error: function(message)
			- logger.warn: function(message)
			- logger.info: function(message)
			- logger.debug: function(message)
			- logger.errorWith: function(message, with_data)
			- logger.warnWith: function(message, with_data)
			- logger.infoWith: function(message, with_data)
			- logger.debugWith: function(message, with_data)

event is the current event, it contains the following:
	- body: Buffer (*not* string, use event.body.toString())
	- content_type: string
	- trigger:
		- class: string
		- kind: string
	- fields: object of field->value
	- headers: object of header->value
	- id: string
	- method: string
	- path: string
	- size: int
	- timestamp: Date
	- url: string
	- version: int


If you use your own base image, make sure to set NODE_PATH.
In nuclio/handler-nodejs image NODE_PATH is set to /usr/local/lib/node_modules
*/
package nodejs
