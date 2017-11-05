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
from datetime import datetime
import cffi
import httplib
import json
import re

ffi = cffi.FFI()
ffi.cdef('''
extern char *strdup (const char *s);

// This must be in sync with interface.h

typedef struct {
  char *body;
  char *content_type;
  long long status_code;
  // TODO: headers
  char *error;
} response_t;

struct API {
    response_t* (*handle_event)(void *event);
    char *(*set_handler)(char *handler);

    // Event interface
    long int (*eventVersion)(void *ptr);
    char * (*eventID)(void *ptr);
    char* (*eventTriggerClass)(void *ptr);
    char* (*eventTriggerKind)(void *ptr);
    char* (*eventContentType)(void *ptr);
    char* (*eventBody)(void *ptr);
    long int (*eventSize)(void *ptr);
    char* (*eventHeader)(void *ptr, char *key);
    double (*eventTimestamp)(void *ptr);
    char* (*eventPath)(void *ptr);
    char* (*eventURL)(void *ptr);
    char* (*eventMethod)(void *ptr);
};

''')

api = None
# Load libc
C = ffi.dlopen(None)


def as_string(val):
    return ffi.string(val).decode('utf-8')


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

    @property
    def header(self, key):
        raise NotImplementedError

        # TODO: Memory leak
        cKey = C.strdup(key.encode('utf-8'))
        value = api.eventHeader(self._ptr, cKey)
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
    handler = ffi.string(handler)
    match = re.match('^([\w|-]+(\.[\w|-]+)*):(\w+)$', handler)
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
        event_handler = load_handler(handler)
    except (ImportError, AttributeError) as err:
        error = str(err)

    return C.strdup(error)


Response = namedtuple('Response', 'headers body content_type status_code')
# TODO: data_binding
Context = namedtuple('Context', 'logger data_binding Response')
context = Context(None, None, Response)


def parse_handler_output(output):
    if isinstance(output, basestring):
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


@ffi.callback('response_t* (void *)')
def handle_event(ptr):
    event._ptr = ptr

    output = event_handler(context, event)
    response = ffi.new('response_t *')

    try:
        output = parse_handler_output(output)
        response[0].body = C.strdup(output.body.encode('utf-8'))
        response[0].content_type = C.strdup(output.content_type)
        response[0].status_code = output.status_code
    except TypeError as err:
        response[0].error = C.strdup(str(err))

    return response


def fill_api(ptr):
    global api

    api = ffi.cast("struct API*", ptr)

    api.handle_event = handle_event
    api.set_handler = set_handler
