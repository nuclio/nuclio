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

/* nuclio.TriggerInfo type */

#include <Python.h>

// This include *must* come after the Python.h include
#include "structmember.h"

typedef struct {
    PyObject_HEAD

        PyObject *class;
    PyObject *kind;
} NuclioTriggerInfo;

static void NuclioTriggerInfo_dealloc(NuclioTriggerInfo *self) {
    Py_XDECREF(self->class);
    self->class = NULL;
    Py_XDECREF(self->kind);
    self->kind = NULL;

    Py_TYPE(self)->tp_free((PyObject *)self);
}

static PyObject *NuclioTriggerInfo_new(PyTypeObject *type, PyObject *args,
                                       PyObject *kwds) {
    NuclioTriggerInfo *self;

    self = (NuclioTriggerInfo *)type->tp_alloc(type, 0);
    return (PyObject *)self;
}

static int NuclioTriggerInfo_init(NuclioTriggerInfo *self, PyObject *args,
                                  PyObject *kwds) {
    PyObject *class = NULL, *kind = NULL;

    if (!PyArg_ParseTuple(args, "OO", &class, &kind)) {
        return -1;
    }

    Py_INCREF(class);
    self->class = class;
    Py_INCREF(kind);
    self->kind = kind;

    return 0;
}

static PyMemberDef NuclioTriggerInfo_members[] = {
    /* class is a keyword in Python */
    {"klass", T_OBJECT_EX, offsetof(NuclioTriggerInfo, class), 0,
     "Trigger class"},
    {"kind", T_OBJECT_EX, offsetof(NuclioTriggerInfo, kind), 0, "Trigger kind"},
    {NULL} /* Sentinel */
};

static PyTypeObject NuclioTriggerInfo_Type = {
    PyVarObject_HEAD_INIT(NULL, 0)

        "nuclio.TriggerInfo",                 /* tp_name */
    sizeof(NuclioTriggerInfo),                /* tp_basicsize */
    0,                                        /* tp_itemsize */
    (destructor)NuclioTriggerInfo_dealloc,    /* tp_dealloc */
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
    "TriggerInfo objects",                    /* tp_doc */
    0,                                        /* tp_traverse */
    0,                                        /* tp_clear */
    0,                                        /* tp_richcompare */
    0,                                        /* tp_weaklistoffset */
    0,                                        /* tp_iter */
    0,                                        /* tp_iternext */
    0,                                        /* tp_methods */
    NuclioTriggerInfo_members,                /* tp_members */
    0,                                        /* tp_getset */
    0,                                        /* tp_base */
    0,                                        /* tp_dict */
    0,                                        /* tp_descr_get */
    0,                                        /* tp_descr_set */
    0,                                        /* tp_dictoffset */
    (initproc)NuclioTriggerInfo_init,         /* tp_init */
    0,                                        /* tp_alloc */
    NuclioTriggerInfo_new,                    /* tp_new */
};

int initialize_trigger_info_type() {
    if (PyType_Ready(&NuclioTriggerInfo_Type) == -1) {
        // TODO
        printf("ERROR: TriggerInfo NOT READY");
        return 0;
    }

    Py_INCREF(&NuclioTriggerInfo_Type);

    return 1;
}

/* Create new nuclio.TriggerInfo from class and kind objects */
PyObject *new_trigger_info(PyObject *class, PyObject *kind) {
    PyObject *args = Py_BuildValue("(O, O)", class, kind);
    PyObject *trigger_info =
        PyObject_CallObject((PyObject *)&NuclioTriggerInfo_Type, args);
    Py_DECREF(args);

    if (PyErr_Occurred() || trigger_info == NULL) {
        PyErr_Print();
        return 0;
    }

    return trigger_info;
}
