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

// FIXME
typedef struct {
  char *body;
  char *content_type;
  long long status_code;
  // TODO: headers
  char *error;
} response_t;

struct API {
    char * (*handle_event)(void *event);
    //response_t (*handle_event)(void *event);
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
C = ffi.dlopen(None)


def as_string(val):
    return ffi.string(val).decode('utf-8')


# We do everything in class level to avoid allocating memory on every event
class event(object):
    _ptr = None

    @classmethod
    def version(cls):
        return api.eventVersion(cls._ptr)

    @classmethod
    def id(cls):
        return as_string(api.eventID(cls._ptr))

    @classmethod
    def trigger_class(cls):
        return as_string(api.eventTriggerClass(cls._ptr))

    @classmethod
    def trigger_kind(cls):
        return as_string(api.eventTriggerKind(cls._ptr))

    @classmethod
    def content_type(cls):
        return as_string(api.eventContentType(cls._ptr))

    @classmethod
    def body(cls):
        return ffi.string(api.eventBody(cls._ptr))

    @classmethod
    def size(cls):
        return api.eventSize(cls._ptr)

    @classmethod
    def header(cls, key):
        raise NotImplementedError

        # TODO: This arrives to Go as empty string
        cKey = ffi.new('char[]', key.encode('utf-8'))
        value = api.eventHeader(cls._ptr, cKey)
        return as_string(value)

    @classmethod
    def timestamp(cls):
        value = api.eventTimestamp(cls._ptr)
        return datetime.fromtimestamp(value)

    @classmethod
    def path(cls):
        return as_string(api.eventPath(cls._ptr))

    @classmethod
    def url(cls):
        return as_string(api.eventURL(cls._ptr))

    @classmethod
    def method(cls):
        return as_string(api.eventMethod(cls._ptr))


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


def c_string(val):
    return ffi.from_buffer(val.encode('utf-8'))


@ffi.callback('char * (char *)')
def set_handler(handler):
    global event_handler

    error = ""
    try:
        event_handler = load_handler(handler)
    except (ImportError, AttributeError) as err:
        error = str(err)

    return c_string(error)


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


#@ffi.callback('response_t (void *)')
@ffi.callback('char * (void *)')
def handle_event(ptr):
    event._ptr = ptr

    output = event_handler(context, event)

    return C.strdup(output)

    response = ffi.new('response_t[]', 1)[0]

    try:
        output = parse_handler_output(output)
        response.body = C.strdup(output.body.encode('utf-8'))
        response.content_type = c_string(output.content_type)
        response.status_code = output.status_code
    except TypeError as err:
        response.error = c_string(str(err))

    return response


def fill_api(ptr):
    global api

    api = ffi.cast("struct API*", ptr)

    api.handle_event = handle_event
    api.set_handler = set_handler
