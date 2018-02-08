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

/* nuclio.Context type */

#include <Python.h>

// This include *must* come after the Python.h include
#include "structmember.h"

extern PyObject *response_type();

typedef struct {
    PyObject_HEAD

        PyObject *logger;
    PyObject *response_type;
} NuclioContext;

static void NuclioContext_dealloc(NuclioContext *self) {
    Py_XDECREF(self->logger);
    self->logger = NULL;
    Py_XDECREF(self->response_type);
    self->response_type = NULL;

    Py_TYPE(self)->tp_free((PyObject *)self);
}

static PyObject *NuclioContext_new(PyTypeObject *type, PyObject *args,
                                   PyObject *kwds) {
    NuclioContext *self;

    self = (NuclioContext *)type->tp_alloc(type, 0);
    self->logger = NULL;
    /* Set context.Response so handler can use it */
    self->response_type = response_type();
    Py_INCREF(self->response_type);

    return (PyObject *)self;
}

static int NuclioContext_init(NuclioContext *self, PyObject *args,
                              PyObject *kwds) {
    PyObject *logger = NULL;

    if (!PyArg_ParseTuple(args, "O", &logger)) {
        return -1;
    }

    self->logger = logger;
    return 0;
}

static PyMemberDef NuclioContext_members[] = {
    {"logger", T_OBJECT_EX, offsetof(NuclioContext, logger), 0,
     "Context logger"},
    {"Response", T_OBJECT_EX, offsetof(NuclioContext, response_type), 0,
     "Response class"},
    {NULL} /* Sentinel */
};

static PyTypeObject NuclioContext_Type = {
    PyVarObject_HEAD_INIT(NULL, 0)

        "nuclio.Context",                     /* tp_name */
    sizeof(NuclioContext),                    /* tp_basicsize */
    0,                                        /* tp_itemsize */
    (destructor)NuclioContext_dealloc,        /* tp_dealloc */
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
    "Context objects",                        /* tp_doc */
    0,                                        /* tp_traverse */
    0,                                        /* tp_clear */
    0,                                        /* tp_richcompare */
    0,                                        /* tp_weaklistoffset */
    0,                                        /* tp_iter */
    0,                                        /* tp_iternext */
    0,                                        /* tp_methods */
    NuclioContext_members,                    /* tp_members */
    0,                                        /* tp_getset */
    0,                                        /* tp_base */
    0,                                        /* tp_dict */
    0,                                        /* tp_descr_get */
    0,                                        /* tp_descr_set */
    0,                                        /* tp_dictoffset */
    (initproc)NuclioContext_init,             /* tp_init */
    0,                                        /* tp_alloc */
    NuclioContext_new,                        /* tp_new */
};

int initialize_context_type() {
    if (PyType_Ready(&NuclioContext_Type) == -1) {
        // TODO
        printf("ERROR: Context NOT READY");
        return 0;
    }

    Py_INCREF(&NuclioContext_Type);

    return 1;
}

/* Create new context */
PyObject *new_context(PyObject *logger) {
    PyObject *args = Py_BuildValue("(O)", logger);
    PyObject *context =
        PyObject_CallObject((PyObject *)&NuclioContext_Type, args);
    Py_DECREF(args);

    if (PyErr_Occurred() || context == NULL) {
        return NULL;
    }

    return context;
}
