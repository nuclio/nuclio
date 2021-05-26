# Copyright 2021 The Nuclio Authors.
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

import asyncio

import aiofile


async def handler(context, event):
    body_str = event.body.decode('utf-8')

    if body_str == 'sleep':
        await asyncio.sleep(0, loop=context.event_loop)
        return 'slept'

    if body_str == 'async_write':
        async def write_async():
            write_mode = 'wb' if isinstance(event.method, bytes) else 'w'
            async with aiofile.async_open('./async_write', write_mode) as w:
                await w.write(event.method)

        asyncio.get_event_loop().create_task(write_async())
        return 'written'

    if body_str == 'read_async_write':
        async with aiofile.async_open('./async_write', 'r') as r:
            return await r.read()

    raise RuntimeError('Unknown return mode: {0}'.format(body_str))
