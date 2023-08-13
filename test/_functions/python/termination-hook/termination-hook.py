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
import time
import os


class TerminationHandler:
    file_path_template = '/tmp/nuclio/termination-hook-{}.txt'

    def __init__(self, worker_id, logger):
        self.worker_id = worker_id
        self.logger = logger
        self.file_name = self.file_path_template.format(worker_id)

    def write_results(self):

        # if the file doesn't exist, create it
        if not os.path.exists(self.file_name):
            self.logger.info_with('Creating file', file_name=self.file_name)
            os.makedirs(os.path.dirname(self.file_name), exist_ok=True)
            open(self.file_name, 'w').close()

        with open(self.file_name, 'a') as f:
            self.logger.info_with('Writing to file', file_name=self.file_name)
            f.write(f'Worker {self.worker_id} - Done!\n')


def init_context(context):
    context.logger.info_with('Initializing', worker_id=context.worker_id)
    termination_handler = TerminationHandler(context.worker_id, logger=context.logger)

    # register a callback to be called when the function is terminated
    context.platform.set_termination_callback(termination_handler.write_results)


def handler(context, event):
    context.logger.info_with('Got event!')

    # simulate a long running function
    sleep_time = 30
    context.logger.info_with('Sleeping', seconds=sleep_time)
    time.sleep(sleep_time)

    context.logger.info_with('Done!')

    return context.Response(body='Done\n', headers={})
