# Copyright 2025 The Nuclio Authors.
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

import time
import asyncio


def my_generator(context, event):
    context.logger.info(f"Streaming response for event {event.path}")
    for i in range(1, 6):
        context.logger.info(f"Sleeping 0.1 second for {i}...")
        time.sleep(0.11)
        context.logger.info(f"Yielding {i}")
        yield str(i)
    context.logger.info(f"Done streaming request")


async def my_async_generator(context, event):
    context.logger.info(f"Streaming async response for event {event.path}")
    for i in range(1, 6):
        context.logger.info(f"Sleeping 0.1 second for {i}...")
        await asyncio.sleep(0.1)
        context.logger.info(f"Yielding {i}")
        yield str(i)
    context.logger.info(f"Done async streaming request")


# Sync handler for sync trigger mode
def handler(context, event):
    if "/generator" in event.path:
        return my_generator(context, event)
    elif "/async-generator" in event.path:
        return my_async_generator(context, event)
    else:
        context.logger.info("Returning simple response")
        return "Simple response"


# Async handler for async trigger mode
async def async_handler(context, event):
    if "/generator" in event.path:
        return my_generator(context, event)
    elif "/async-generator" in event.path:
        return my_async_generator(context, event)
    else:
        context.logger.info("Returning simple response")
        return "Simple response"

