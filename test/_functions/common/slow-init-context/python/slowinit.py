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

# Regression test function for the async-mode slow init_context bug.
#
# Before the fix, the async wrapper only bound its TCP server socket inside
# start(), which runs after initialize() (and therefore after init_context).
# Go's ConnectionAllocator has a ~30 s retry window; any init_context longer
# than that caused "dial tcp 127.0.0.1:1337: connect: connection refused".
#
# After the fix the socket is bound in __init__ before init_context runs, so
# Go can connect immediately while initialization is still in progress.

import time


_INIT_SLEEP_SECONDS = 35


def init_context(context):
    context.logger.info(f"Starting slow init_context, sleeping {_INIT_SLEEP_SECONDS}s")
    time.sleep(_INIT_SLEEP_SECONDS)
    context.logger.info("Slow init_context complete")


async def handler(context, event):
    return context.Response(body="ok", status_code=200)
