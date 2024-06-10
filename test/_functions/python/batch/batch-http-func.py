# Copyright 2023 The Nuclio Authors.
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


def handler(context, event: list[nuclio_sdk.Event]):
    if isinstance(event, list):
        context.logger.info_with('Got batched event!')
        x = 0
        batched_response = []
        for item in event:
            event_id = item.id
            batched_response.append(nuclio_sdk.Response(
                body="Response to in-batch event",
                headers={},
                content_type="text",
                status_code=200,
                event_id=event_id,
            ))
        return batched_response
    else:
        context.logger.info_with('Got single event!')
        return nuclio_sdk.Response(
            body=str(123),
            headers={},
            content_type="text",
            status_code=200,
            event_id=0,
        )