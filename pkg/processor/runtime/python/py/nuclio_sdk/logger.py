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

import logging

import nuclio_sdk.json_encoder


class JSONFormatter(logging.Formatter):
    def __init__(self):
        super(JSONFormatter, self).__init__()

        self._json_encoder = nuclio_sdk.json_encoder.Encoder()

    def format(self, record):
        record_fields = {
            'datetime': self.formatTime(record, self.datefmt),
            'level': record.levelname.lower(),
            'message': record.getMessage(),
            'with': getattr(record, 'with', {}),
        }

        return 'l' + self._json_encoder.encode(record_fields)


class HumanReadableFormatter(logging.Formatter):

    def __init__(self):
        super(HumanReadableFormatter, self).__init__()

    def format(self, record):
        record_with = getattr(record, 'with', {})
        if record_with:
            more = ': {0}'.format(record_with)
        else:
            more = ''

        return 'Python> {0} [{1}] {2}{3}'.format(self.formatTime(record, self.datefmt),
                                                 record.levelname.lower(),
                                                 record.getMessage(),
                                                 more)


class Logger(object):

    def __init__(self, level):
        self._logger = logging.getLogger('nuclio_sdk')
        self._logger.setLevel(level)
        self._handlers = {}

    def set_handler(self, handler_name, file, formatter):

        # check if there's a handler by this name
        if handler_name in self._handlers:

            # log that we're removing it
            self.info_with('Replacing logger output')

            self._logger.removeHandler(self._handlers[handler_name])

        # create a stream handler from the file
        stream_handler = logging.StreamHandler(file)

        # set the formatter
        stream_handler.setFormatter(formatter)

        # add the handler to the logger
        self._logger.addHandler(stream_handler)

        # save as the named output
        self._handlers[handler_name] = stream_handler

    def debug(self, message, *args):
        self._logger.debug(message, *args)

    def info(self, message, *args):
        self._logger.info(message, *args)

    def warn(self, message, *args):
        self._logger.warning(message, *args)

    def error(self, message, *args):
        self._logger.error(message, *args)

    def debug_with(self, message, *args, **kw_args):
        self._logger.debug(message, *args, extra={'with': kw_args})

    def info_with(self, message, *args, **kw_args):
        self._logger.info(message, *args, extra={'with': kw_args})

    def warn_with(self, message, *args, **kw_args):
        self._logger.warning(message, *args, extra={'with': kw_args})

    def error_with(self, message, *args, **kw_args):
        self._logger.error(message, *args, extra={'with': kw_args})
