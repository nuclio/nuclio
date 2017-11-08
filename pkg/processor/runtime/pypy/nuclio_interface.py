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

from collections import namedtuple
from contextlib import contextmanager
from datetime import datetime
import cffi
import logging
import httplib
import json
import re

ffi = cffi.FFI()
ffi.cdef('''
extern char *strdup (const char *s);
extern void free (void *);

// This must be in sync with interface.h

typedef struct {
  char *body;
  char *content_type;
  long long status_code;
  // TODO: headers
  char *error;
} response_t;

struct API {
    response_t* (*handle_event)(void *context, void *event);
    char *(*set_handler)(char *handler);

    // Event interface
    long int (*eventVersion)(void *ptr);
    char * (*eventID)(void *ptr);
    char* (*eventTriggerClass)(void *ptr);
    char* (*eventTriggerKind)(void *ptr);
    char* (*eventContentType)(void *ptr);
    char* (*eventBody)(void *ptr);
    long int (*eventSize)(void *ptr);
    char* (*eventHeaderString)(void *ptr, char *key);
    char* (*eventFieldString)(void *ptr, char *key);
    double (*eventTimestamp)(void *ptr);
    char* (*eventPath)(void *ptr);
    char* (*eventURL)(void *ptr);
    char* (*eventMethod)(void *ptr);

    void (*contextLogInfo)(void *, char *);
    void (*contextLogError)(void *, char *);
    void (*contextLogWarn)(void *, char *);
    void (*contextLogDebug)(void *, char *);

    void (*contextLogErrorWith)(void *, char *, char *);
    void (*contextLogWarnWith)(void *, char *, char *);
    void (*contextLogInfoWith)(void *, char *, char *);
    void (*contextLogDebugWith)(void *, char *, char *);
};

''')

api = None
# Load libc
C = ffi.dlopen(None)


def as_string(val):
    return ffi.string(val).decode('utf-8')


@contextmanager
def c_string(val):
    c_val = C.strdup(val.encode('utf-8'))
    try:
        yield c_val
    finally:
        C.free(c_val)


class Event(object):
    # We have only one event per interpreter to avoid memory allocations
    _ptr = None

    @property
    def version(self):
        return api.eventVersion(self._ptr)

    @property
    def id(self):
        return as_string(api.eventID(self._ptr))

    @property
    def trigger_class(self):
        return as_string(api.eventTriggerClass(self._ptr))

    @property
    def trigger_kind(self):
        return as_string(api.eventTriggerKind(self._ptr))

    @property
    def content_type(self):
        return as_string(api.eventContentType(self._ptr))

    @property
    def body(self):
        return ffi.string(api.eventBody(self._ptr))

    @property
    def size(self):
        return api.eventSize(self._ptr)

    # TODO: Make this API more Pythoninc
    # headers and fields attributes which are dict like: event.headers['key']
    def header(self, key):
        with c_string(key) as c_key:
            value = api.eventHeaderString(self._ptr, c_key)

        return as_string(value)

    def field(self, key):
        with c_string(key) as c_key:
            value = api.eventFieldString(self._ptr, c_key)

        return as_string(value)

    @property
    def timestamp(self):
        value = api.eventTimestamp(self._ptr)
        return datetime.fromtimestamp(value)

    @property
    def path(self):
        return as_string(api.eventPath(self._ptr))

    @property
    def url(self):
        return as_string(api.eventURL(self._ptr))

    @property
    def method(self):
        return as_string(api.eventMethod(self._ptr))


event = Event()
event_handler = None


def load_module(name):
    """Load module in the format 'json.tool'"""
    mod = __import__(name)
    for sub in name.split('.')[1:]:
        mod = getattr(mod, sub)
    return mod


def load_handler(handler):
    """Load handler function from handler.

    handler is in the format 'module.sub:handler_name'
    """
    # 'app.handler:reverser'
    match = re.match('^(\w+(\.\w+)*):(\w+)$', handler)
    if not match:
        raise ValueError('malformed handler')

    mod_name, func_name = match.group(1), match.group(3)
    mod = load_module(mod_name)

    return getattr(mod, func_name)


@ffi.callback('char * (char *)')
def set_handler(handler):
    global event_handler

    error = ""
    try:
        handler = ffi.string(handler)
        event_handler = load_handler(handler)
    except (ImportError, AttributeError, ValueError) as err:
        error = str(err)

    return C.strdup(error)


Response = namedtuple('Response', 'headers body content_type status_code')


class NuclioHandler(logging.Handler):
    # Will be populated in fill_api
    levelMapping = None
    levelMappingWith = None

    def emit(self, record):
        if not context._ptr:
            # TODO: Log somehow?
            return

        message = self.format(record)
        with_data = getattr(record, 'with')
        if with_data:
            with_data = json.dumps(with_data).encode('utf-8')

            log_func = {
                logging.CRITICAL: api.contextLogErrorWith,
                logging.ERROR: api.contextLogErrorWith,
                logging.WARNING: api.contextLogWarnWith,
                logging.INFO: api.contextLogInfoWith,
                logging.DEBUG: api.contextLogDebugWith,
            }.get(record.levelno, api.contextLogInfoWith)

            with c_string(message) as c_message, c_string(with_data) as c_with:
                log_func(context._ptr, c_message, c_with)
        else:
            log_func = {
                logging.CRITICAL: api.contextLogError,
                logging.ERROR: api.contextLogError,
                logging.WARNING: api.contextLogWarn,
                logging.INFO: api.contextLogInfo,
                logging.DEBUG: api.contextLogDebug,
            }.get(record.levelno, api.contextLogInfo)
            with c_string(message) as c_message:
                log_func(context._ptr, c_message)


class Context(object):
    # We have only one context per interpreter to avoid memory allocations
    _ptr = None

    def __init__(self):
        self.logger = self._create_logger()
        self.Response = Response
        # TODO
        self.data_binding = None

    def _create_logger(self):
        log = logging.getLogger('nuclio/pypy')
        log.setLevel(logging.DEBUG)  # TODO: Get from environment?
        handler = NuclioHandler()
        handler.setFormatter(logging.Formatter('%(message)s'))
        log.addHandler(NuclioHandler())

        for name in ['critical', 'fatal', 'error', 'warning', 'info', 'debug']:
            self.add_structured_log_method(log, name)

        return log

    def add_structured_log_method(self, logger, name):
        """Add a `<name>_with` method to logger.

        This will populate the `extra` parameter with `with` key
        """
        method = getattr(logger, name)

        def with_method(message, *args, **kw):
            method(message, *args, extra={'with': kw})

        setattr(logger, '{}_with'.format(name), with_method)


context = Context()


def parse_handler_output(output):
    if isinstance(output, basestring):  # noqa
        return Response(
            body=output,
            content_type='',
            status_code=httplib.OK,
            headers={},
        )

    if isinstance(output, tuple) and len(output) == 2:
        return Response(
            status_code=output[0],
            body=output[1],
            content_type='',
            headers={},
        )

    if isinstance(output, (dict, list)):
        return Response(
            body=json.dumps(output),
            content_type='application/json',
            status_code=httplib.OK,
            headers={},
        )

    if isinstance(output, Response):
        return output

    raise TypeError('unknown output type - {}'.format(type(output)))


@ffi.callback('response_t* (void *, void *)')
def handle_event(context_ptr, event_ptr):
    context._ptr = context_ptr
    event._ptr = event_ptr

    response = ffi.new('response_t *')
    try:
        output = event_handler(context, event)
        output = parse_handler_output(output)

        response[0].body = C.strdup(output.body.encode('utf-8'))
        response[0].content_type = C.strdup(output.content_type)
        response[0].status_code = output.status_code
    # We can't predict exceptions in user handler code so we catch everything
    except Exception as err:
        response[0].error = C.strdup(str(err))

    return response


def fill_api(ptr):
    global api

    api = ffi.cast("struct API*", ptr)

    api.handle_event = handle_event
    api.set_handler = set_handler
