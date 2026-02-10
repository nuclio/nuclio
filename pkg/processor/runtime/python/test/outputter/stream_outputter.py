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
import asyncio
import os

file_path = "/tmp/stream_outputter_lines.txt"

def init_context(context):
    # Write 50 lines to the file
    with open(file_path, "w") as f:
        for i in range(1, 51):
            f.write(f"Line {i}\n")

async def stream_file_lines_async(context, event):
    # Stream the file line by line asynchronously
    import aiofile
    async with aiofile.AIOFile(file_path, "r") as afp:
        async for line in aiofile.LineReader(afp):
            yield line

async def stream_file_lines_as_response_async(context, event):
    return context.Response(body=stream_file_lines_async(context, event), status_code=202, content_type="text/custom")

async def stream_file_lines_sync_as_async(context, event):
    with open(file_path, "r") as f:
        for line in f:
            yield line

async def stream_file_lines_as_response_sync_as_async(context, event):
    return context.Response(body=stream_file_lines_sync(context, event), status_code=202, content_type="text/custom")

def stream_file_lines_sync(context, event):
    with open(file_path, "r") as f:
        for line in f:
            yield line

def stream_file_lines_as_response_sync(context, event):
    return context.Response(body=stream_file_lines_sync(context, event),  status_code=202, content_type="text/custom")

async def stream_single_yield_async(context, event):
    """Stream handler that yields only once - tests END_OF_STREAM behavior for single yield"""
    yield "single_chunk"

async def stream_single_yield_sync_as_async(context, event):
    """Sync generator defined as async def that yields only once"""
    yield "single_chunk"


async def stream_flush_test(context, event):
    """Yields chunks with delays to test periodic flush. With 1s flush period, client should see chunks incrementally."""
    yield "flush1"
    await asyncio.sleep(0.6)
    yield "flush2"
    await asyncio.sleep(0.6)
    yield "flush3"


async def stream_flush_test_as_response(context, event):
    return context.Response(
        body=stream_flush_test(context, event),
        status_code=200,
        content_type="text/plain",
    )
