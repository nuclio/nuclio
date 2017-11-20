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

from collections import namedtuple, Mapping
from contextlib import contextmanager
from datetime import datetime
from functools import partial
from traceback import format_exc
import cffi
import httplib
import json
import logging
import re
import threading


ffi = cffi.FFI()
ffi.cdef('''
extern char *strdup (const char *s);
extern void free (void *);

// This must be in sync with interface.h

typedef struct {
  char *body;
  char *content_type;
  long long status_code;
  char *headers;
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
    char* (*eventHeaders)(void *ptr);
    char* (*eventFields)(void *ptr);
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


class GoMap(Mapping):
    def __init__(self, get_func):
        self._get_func = get_func
        self._dict = None

    def _fetch(self):
        if self._dict is not None:
            return

        self._dict = self._get_func() or {}
        # Case insensitive
        self._lower2key = {key.lower(): key for key in self._dict}

    def __str__(self):
        self._fetch()
        return str(self._dict)

    def __repr__(self):
        self._fetch()
        return repr(self._dict)

    def __getitem__(self, key):
        self._fetch()

        orig_key = self._lower2key.get(key.lower())
        if orig_key is None:
            raise KeyError(key)

        return self._dict[orig_key]

    def __iter__(self):
        self._fetch()
        return iter(self._dict)

    def __len__(self):
        self._fetch()
        return len(self._dict)


class Event(object):
    # We have only one event per interpreter to avoid memory allocations
    _ptr = None

    def __init__(self):
        self.headers = GoMap(partial(self._get_json, api.eventHeaders))
        self.fields = GoMap(partial(self._get_json, api.eventFields))

    def _get_json(self, fn):
        val = as_string(fn(self._ptr))
        try:
            return json.loads(val)
        except ValueError as err:
            ctx = get_context()
            ctx.logger.error('cannot parse json from %r - %s', val, err)
            return {}

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


_event_handler = None


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
    global _event_handler

    error = ''
    try:
        handler = ffi.string(handler)
        _event_handler = load_handler(handler)
    except (ImportError, AttributeError, ValueError) as err:
        error = str(err)

    return C.strdup(error)


Response = namedtuple('Response', 'headers body content_type status_code')


class NuclioHandler(logging.Handler):
    # Will be populated in fill_api
    levelMapping = None
    levelMappingWith = None

    def emit(self, record):
        context = get_context()

        if not context._ptr:
            # TODO: Log somehow?
            return

        message = self.format(record)
        with_data = getattr(record, 'with', None)
        if with_data:
            with_data = json.dumps(with_data).encode('utf-8')

            log_func = {
                # FATAL == CRITICAL
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
                # FATAL == CRITICAL
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


def parse_handler_output(output):
    if isinstance(output, basestring):  # noqa
        return Response(
            headers={},
            body=output,
            content_type='',
            status_code=httplib.OK,
        )

    if isinstance(output, tuple) and len(output) == 2:
        body = output[1]
        content_type = ''

        if not isinstance(body, basestring):
            body = json.dumps(body)
            content_type = 'application/json'

        return Response(
            headers={},
            body=body,
            content_type=content_type,
            status_code=output[0],
        )

    if isinstance(output, (dict, list)):
        return Response(
            headers={},
            body=json.dumps(output),
            content_type='application/json',
            status_code=httplib.OK,
        )

    if isinstance(output, Response):
        return output

    raise TypeError('unknown output type - {}'.format(type(output)))


# Thread safty, store event, context and response per thread
tls = threading.local()


def get_event():
    try:
        return tls.event
    except AttributeError:
        event = tls.event = Event()
        return event


def get_context():
    try:
        return tls.context
    except AttributeError:
        context = tls.context = Context()
        return context


def get_response():
    try:
        response = tls.response
    except AttributeError:
        response = tls.response = ffi.new('response_t *')

    if response[0].body != ffi.NULL:
        C.free(response[0].body)
        response[0].body = ffi.NULL

    if response[0].content_type != ffi.NULL:
        C.free(response[0].content_type)
        response[0].content_type = ffi.NULL

    if response[0].headers != ffi.NULL:
        C.free(response[0].headers)
        response[0].headers = ffi.NULL

    if response[0].error != ffi.NULL:
        C.free(response[0].error)
        response[0].error = ffi.NULL

    return response


@ffi.callback('response_t* (void *, void *)')
def handle_event(context_ptr, event_ptr):
    context = get_context()
    event = get_event()

    context._ptr = context_ptr
    event._ptr = event_ptr

    response = get_response()
    try:
        output = _event_handler(context, event)
        output = parse_handler_output(output)

        response[0].body = C.strdup(output.body.encode('utf-8'))
        response[0].content_type = C.strdup(output.content_type)
        headers = json.dumps(output.headers).encode('utf-8')
        response[0].headers = C.strdup(headers)
        response[0].status_code = output.status_code
    # We can't predict exceptions in user handler code so we catch everything
    except Exception as err:
        context.logger.error_with(
            'error in handler', error=str(err), traceback=format_exc())
        response[0].headers = C.strdup('{}'.encode('utf-8'))
        response[0].status_code = httplib.INTERNAL_SERVER_ERROR
        response[0].error = C.strdup(str(err))

    return response


def fill_api(ptr):
    global api

    api = ffi.cast('struct API*', ptr)

    api.handle_event = handle_event
    api.set_handler = set_handler
