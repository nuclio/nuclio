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
import os
import aiofile

file_path = "/tmp/stream_outputter_lines.txt"

def init_context(context):
    # Write 50 lines to the file
    with open(file_path, "w") as f:
        for i in range(1, 51):
            f.write(f"Line {i}\n")

async def stream_file_lines_async(context, event):
    # Stream the file line by line asynchronously
    async with aiofile.AIOFile(file_path, "r") as afp:
        async for line in aiofile.LineReader(afp):
            yield line

async def stream_file_lines_as_response_async(context, event):
    return context.Response(body=stream_file_lines_async(context, event))

async def stream_file_lines_sync(context, event):
    with open(file_path, "r") as f:
        for line in f:
            yield line

async def stream_file_lines_as_response_sync(context, event):
    return context.Response(body=stream_file_lines_sync(context, event))