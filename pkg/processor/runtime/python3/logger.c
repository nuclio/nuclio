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

/* nuclio.Logger type */

#include <Python.h>

#include <stdio.h>
#include "_cgo_export.h"
// This include *must* come after the Python.h include
#include "structmember.h"
#include "types.h"

#define CHECK_LOGGER(logger_ptr)                                       \
    if ((logger_ptr) == 0) {                                           \
        PyErr_SetString(PyExc_AttributeError, "Uninitialized logger"); \
        return NULL;                                                   \
    }

typedef struct {
    PyObject_HEAD

        unsigned long logger_ptr;
} NuclioLogger;

static void NuclioLogger_dealloc(NuclioLogger *self) {
    Py_TYPE(self)->tp_free((PyObject *)self);
}

static PyObject *NuclioLogger_new(PyTypeObject *type, PyObject *args,
                                  PyObject *kwds) {
    NuclioLogger *self;

    self = (NuclioLogger *)type->tp_alloc(type, 0);
    self->logger_ptr = 0;
    return (PyObject *)self;
}

static int NuclioLogger_init(NuclioLogger *self, PyObject *args,
                             PyObject *kwds) {
    // logger_ptr is initialized by new_logger
    return 0;
}

static PyObject *NuclioLogger_log(NuclioLogger *self, PyObject *args,
                                  int level) {
    PyObject *message = NULL;

    if (!PyArg_ParseTuple(args, "O", &message)) {
        return NULL;
    }

    char *cMessage = PyUnicode_AsUTF8(message);
    loggerLog(self->logger_ptr, level, cMessage);
    // No need to free cMessage

    Py_RETURN_NONE;
}

static PyObject *NuclioLogger_error(NuclioLogger *self, PyObject *args) {
    return NuclioLogger_log(self, args, LOG_LEVEL_ERROR);
}

static PyObject *NuclioLogger_warning(NuclioLogger *self, PyObject *args) {
    return NuclioLogger_log(self, args, LOG_LEVEL_WARNING);
}

static PyObject *NuclioLogger_info(NuclioLogger *self, PyObject *args) {
    return NuclioLogger_log(self, args, LOG_LEVEL_INFO);
}

static PyObject *NuclioLogger_debug(NuclioLogger *self, PyObject *args) {
    return NuclioLogger_log(self, args, LOG_LEVEL_DEBUG);
}

static PyObject *NuclioLogger_log_with(NuclioLogger *self, PyObject *args,
                                       PyObject *kw, int level) {
    PyObject *message = NULL;

    if (!PyArg_ParseTuple(args, "O", &message)) {
        return NULL;
    }

    char *cMessage = PyUnicode_AsUTF8(message);
    loggerLogWith(self->logger_ptr, level, cMessage, kw);

    // TODO: Do we need to Py_XDECREF(kw) ?

    Py_RETURN_NONE;
}

static PyObject *NuclioLogger_error_with(NuclioLogger *self, PyObject *args,
                                         PyObject *kw) {
    return NuclioLogger_log_with(self, args, kw, LOG_LEVEL_ERROR);
}

static PyObject *NuclioLogger_warning_with(NuclioLogger *self, PyObject *args,
                                           PyObject *kw) {
    return NuclioLogger_log_with(self, args, kw, LOG_LEVEL_WARNING);
}

static PyObject *NuclioLogger_info_with(NuclioLogger *self, PyObject *args,
                                        PyObject *kw) {
    return NuclioLogger_log_with(self, args, kw, LOG_LEVEL_INFO);
}

static PyObject *NuclioLogger_debug_with(NuclioLogger *self, PyObject *args,
                                         PyObject *kw) {
    return NuclioLogger_log_with(self, args, kw, LOG_LEVEL_DEBUG);
}

static PyMethodDef NuclioLogger_methods[] = {
    {"error", (PyCFunction)NuclioLogger_error, METH_VARARGS, "error log"},
    {"info", (PyCFunction)NuclioLogger_info, METH_VARARGS, "Info log"},
    {"warning", (PyCFunction)NuclioLogger_warning, METH_VARARGS, "warning log"},
    {"debug", (PyCFunction)NuclioLogger_debug, METH_VARARGS, "debug log"},
    {"error_with", (PyCFunction)NuclioLogger_error_with,
     METH_VARARGS | METH_KEYWORDS, "error log with parameters"},
    {"warning_with", (PyCFunction)NuclioLogger_warning_with,
     METH_VARARGS | METH_KEYWORDS, "warning log with parameters"},
    {"info_with", (PyCFunction)NuclioLogger_info_with,
     METH_VARARGS | METH_KEYWORDS, "info log with parameters"},
    {"debug_with", (PyCFunction)NuclioLogger_debug_with,
     METH_VARARGS | METH_KEYWORDS, "debug log with parameters"},
    {NULL}  // Sentinel
};

static PyTypeObject NuclioLogger_Type = {
    PyVarObject_HEAD_INIT(NULL, 0)

        "nuclio.Logger",                      /* tp_name */
    sizeof(NuclioLogger),                     /* tp_basicsize */
    0,                                        /* tp_itemsize */
    (destructor)NuclioLogger_dealloc,         /* tp_dealloc */
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
    NuclioLogger_methods,                     /* tp_methods */
    0,                                        /* tp_members */
    0,                                        /* tp_getset */
    0,                                        /* tp_base */
    0,                                        /* tp_dict */
    0,                                        /* tp_descr_get */
    0,                                        /* tp_descr_set */
    0,                                        /* tp_dictoffset */
    (initproc)NuclioLogger_init,              /* tp_init */
    0,                                        /* tp_alloc */
    NuclioLogger_new,                         /* tp_new */
};

int initialize_logger_type() {
    if (PyType_Ready(&NuclioLogger_Type) == -1) {
        return 0;
    }

    Py_INCREF(&NuclioLogger_Type);

    return 1;
}

/* Create new nuclio.Logger object and set logger_ptr */
PyObject *new_logger(unsigned long logger_ptr) {
    PyObject *logger =
        PyObject_CallObject((PyObject *)&NuclioLogger_Type, NULL);
    if (PyErr_Occurred() || logger == NULL) {
        return NULL;
    }

    ((NuclioLogger *)logger)->logger_ptr = logger_ptr;
    return logger;
}
