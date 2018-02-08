// +build python3

/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/* nuclio.Event type */

#include <Python.h>

#include "_cgo_export.h"
// This include *must* come after the Python.h include
#include "structmember.h"

#include <stdio.h>

#define CHECK_EVENT(event_ptr)                                        \
    if ((event_ptr) == 0) {                                           \
        PyErr_SetString(PyExc_AttributeError, "Uninitialized event"); \
        return NULL;                                                  \
    }

typedef struct {
    PyObject_HEAD

        unsigned long event_ptr;
    // TODO: Cache more attributes once we get them from Go
    PyObject *headers;
    PyObject *fields;
} NuclioEvent;

static void NuclioEvent_dealloc(NuclioEvent *self) {
    if (self->headers != NULL) {
        Py_DECREF(self->headers);
        self->headers = NULL;
    }

    if (self->fields != NULL) {
        Py_DECREF(self->fields);
        self->fields = NULL;
    }

    Py_TYPE(self)->tp_free((PyObject *)self);
}

static PyObject *NuclioEvent_new(PyTypeObject *type, PyObject *args,
                                 PyObject *kwds) {
    NuclioEvent *self;

    self = (NuclioEvent *)type->tp_alloc(type, 0);
    self->event_ptr = 0;
    self->headers = NULL;
    self->fields = NULL;
    return (PyObject *)self;
}

static int NuclioEvent_init(NuclioEvent *self, PyObject *args, PyObject *kwds) {
    // event_ptr is initialized by new_event
    return 0;
}

static PyObject *NuclioEvent_getid(NuclioEvent *self, void *closure) {
    CHECK_EVENT(self->event_ptr);
    return eventID(self->event_ptr);
}

static PyObject *NuclioEvent_gettrigger(NuclioEvent *self, void *closure) {
    CHECK_EVENT(self->event_ptr);
    return eventTriggerInfo(self->event_ptr);
}

static PyObject *NuclioEvent_getcontent_type(NuclioEvent *self, void *closure) {
    CHECK_EVENT(self->event_ptr);
    return eventContentType(self->event_ptr);
}

static PyObject *NuclioEvent_getbody(NuclioEvent *self, void *closure) {
    CHECK_EVENT(self->event_ptr);
    return eventBody(self->event_ptr);
}

static PyObject *NuclioEvent_getheaders(NuclioEvent *self, void *closure) {
    CHECK_EVENT(self->event_ptr);

    if (self->headers == NULL) {
        self->headers = eventHeaders(self->event_ptr);
    }

    Py_INCREF(self->headers);
    return self->headers;
}

static PyObject *NuclioEvent_getfields(NuclioEvent *self, void *closure) {
    CHECK_EVENT(self->event_ptr);

    if (self->fields == NULL) {
        self->fields = eventFields(self->event_ptr);
    }

    Py_INCREF(self->fields);
    return self->fields;
}

static PyObject *NuclioEvent_gettimestamp(NuclioEvent *self, void *closure) {
    CHECK_EVENT(self->event_ptr);
    return eventTimestamp(self->event_ptr);
}

static PyObject *NuclioEvent_getpath(NuclioEvent *self, void *closure) {
    CHECK_EVENT(self->event_ptr);
    return eventPath(self->event_ptr);
}

static PyObject *NuclioEvent_geturl(NuclioEvent *self, void *closure) {
    CHECK_EVENT(self->event_ptr);
    return eventURL(self->event_ptr);
}

static PyObject *NuclioEvent_getmethod(NuclioEvent *self, void *closure) {
    CHECK_EVENT(self->event_ptr);
    return eventMethod(self->event_ptr);
}

static PyObject *NuclioEvent_getshard_id(NuclioEvent *self, void *closure) {
    CHECK_EVENT(self->event_ptr);
    return eventShardID(self->event_ptr);
}

static PyObject *NuclioEvent_getnum_shards(NuclioEvent *self, void *closure) {
    CHECK_EVENT(self->event_ptr);
    return eventNumShards(self->event_ptr);
}

static PyGetSetDef NuclioEvent_getsetlist[] = {
    {"id", (getter)NuclioEvent_getid, NULL, NULL, "Event ID"},
    {"trigger", (getter)NuclioEvent_gettrigger, NULL, NULL, "Event trigger"},
    {"content_type", (getter)NuclioEvent_getcontent_type, NULL, NULL,
     "Event content type"},
    {"body", (getter)NuclioEvent_getbody, NULL, NULL, "Event body"},
    {"headers", (getter)NuclioEvent_getheaders, NULL, NULL, "Event headers"},
    {"fields", (getter)NuclioEvent_getfields, NULL, NULL, "Event fields"},
    {"timestamp", (getter)NuclioEvent_gettimestamp, NULL, NULL,
     "Event timestamp"},
    {"path", (getter)NuclioEvent_getpath, NULL, NULL, "Event path"},
    {"url", (getter)NuclioEvent_geturl, NULL, NULL, "Event URL"},
    {"method", (getter)NuclioEvent_getmethod, NULL, NULL, "Event method"},
    {"shard_id", (getter)NuclioEvent_getshard_id, NULL, NULL, "Event shard ID"},
    {"num_shards", (getter)NuclioEvent_getnum_shards, NULL, NULL,
     "Number of Event shards"},
    {NULL}  // Sentinel
};

static PyTypeObject NuclioEvent_Type = {
    PyVarObject_HEAD_INIT(NULL, 0)

        "nuclio.Event",                       /* tp_name */
    sizeof(NuclioEvent),                      /* tp_basicsize */
    0,                                        /* tp_itemsize */
    (destructor)NuclioEvent_dealloc,          /* tp_dealloc */
    0,                                        /* tp_print */
    0,                                        /* tp_getattr */
    0,                                        /* tp_setattr */
    0,                                        /* tp_reserved */
    0,                                        /* tp_repr */
    0,                                        /* tp_as_number */
    0,                                        /* tp_as_sequence */
    0,                                        /* tp_as_mapping */
    0,                                        /* tp_hash  */
    0,                                        /* tp_call */
    0,                                        /* tp_str */
    0,                                        /* tp_getattro */
    0,                                        /* tp_setattro */
    0,                                        /* tp_as_buffer */
    Py_TPFLAGS_DEFAULT | Py_TPFLAGS_BASETYPE, /* tp_flags */
    "Event objects",                          /* tp_doc */
    0,                                        /* tp_traverse */
    0,                                        /* tp_clear */
    0,                                        /* tp_richcompare */
    0,                                        /* tp_weaklistoffset */
    0,                                        /* tp_iter */
    0,                                        /* tp_iternext */
    0,                                        /* tp_methods */
    0,                                        /* tp_members */
    NuclioEvent_getsetlist,                   /* tp_getset */
    0,                                        /* tp_base */
    0,                                        /* tp_dict */
    0,                                        /* tp_descr_get */
    0,                                        /* tp_descr_set */
    0,                                        /* tp_dictoffset */
    (initproc)NuclioEvent_init,               /* tp_init */
    0,                                        /* tp_alloc */
    NuclioEvent_new,                          /* tp_new */
};

int initialize_event_type() {
    if (PyType_Ready(&NuclioEvent_Type) == -1) {
        printf("ERROR: Event NOT READY");
        return 0;
    }

    Py_INCREF(&NuclioEvent_Type);

    return 1;
}

/* Create new nuclio.Event object and set event_ptr */
PyObject *new_event(unsigned long event_ptr) {
    PyObject *event = PyObject_CallObject((PyObject *)&NuclioEvent_Type, NULL);
    if (PyErr_Occurred() || event == NULL) {
        return NULL;
    }

    ((NuclioEvent *)event)->event_ptr = event_ptr;
    return event;
}
