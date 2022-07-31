# Copyright 2017 The Nuclio Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import nuclio_sdk


async def handler(context, event):

    context.logger.info("event.body: {0}\nevent.headers: {1}".format(event.body, event.headers))

    # ensure no ack on response
    response = nuclio_sdk.Response()
    response.ensure_no_ack()

    # if a 100 messages are sent, explicit ack
    body = event.body.decode('utf-8')
    if "99" in body:
        context.logger.info("Received 100th message, sending Explicit ack")
        await context.platform.explicit_ack(event)

    # if event headers contain "x-nuclio-explicit-ack", ack the response
    lower_case_headers = {k.lower(): v for k, v in event.headers.items()}
    if "x-nuclio-explicit-ack" in lower_case_headers and lower_case_headers["x-nuclio-explicit-ack"] == "true":
        context.logger.info("Received a request to ack, sending Explicit ack")
        await context.platform.explicit_ack(event)

    return response
