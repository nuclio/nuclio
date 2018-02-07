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

#include <Python.h>

// This include *must* come after the Python.h include
#include "structmember.h"
#include "types.h"

typedef struct {
    PyObject_HEAD

	PyObject *body;
    PyObject *content_type;
    PyObject *status_code;
    PyObject *headers;

} NuclioResponse;

static void NuclioResponse_dealloc(NuclioResponse *self) {
    Py_XDECREF(self->body);
    self->body = NULL;
    Py_XDECREF(self->status_code);
    self->status_code = NULL;
    Py_XDECREF(self->content_type);
    self->content_type = NULL;
    Py_XDECREF(self->headers);
    self->headers = NULL;

    Py_TYPE(self)->tp_free((PyObject *)self);
}

static PyObject *NuclioResponse_new(PyTypeObject *type, PyObject *args,
				    PyObject *kwds) {
    NuclioResponse *self;

    self = (NuclioResponse *)type->tp_alloc(type, 0);
    self->body = Py_BuildValue("");
    self->status_code = Py_BuildValue("");
    self->content_type = Py_BuildValue("");
    self->headers = Py_BuildValue("");
    return (PyObject *)self;
}

static PyObject *NuclioResponse_getbody(NuclioResponse *self, void *closure) {
    Py_INCREF(self->body);
    return self->body;
}

static int NuclioResponse_setbody(NuclioResponse *self, PyObject *value,
				  void *closure) {
    if ((value == NULL) || (value == Py_None)) {
	Py_XDECREF(self->body);
	self->body = PyBytes_FromStringAndSize(NULL, 0);
	return 0;
    }

    if (PyBytes_Check(value)) {
	Py_INCREF(value);
	self->body = value;
    } else if (PyUnicode_Check(value)) {
	value = PyUnicode_AsEncodedString(value, "UTF-8", "strict");
	if (PyErr_Occurred()) {
	    return -1;
	}
	self->body = value;
	// No need to Py_INCREF since PyUnicode_AsEncodedString return new
	// reference
    } else {
	// TODO: Handle other body types (list, dict ...)
	PyErr_SetString(PyExc_TypeError, "body must be bytes or str");
	return -1;
    }

    return 0;
}

static PyObject *NuclioResponse_getstatus_code(NuclioResponse *self,
					       void *closure) {
    Py_INCREF(self->status_code);
    return self->status_code;
}

static int NuclioResponse_setstatus_code(NuclioResponse *self, PyObject *value,
					 void *closure) {
    if ((value == NULL) || (value == Py_None)) {
	Py_XDECREF(self->status_code);
	self->status_code = Py_BuildValue("");
	return 0;
    }

    if (!PyLong_Check(value)) {
	PyErr_SetString(PyExc_TypeError, "status_code must be an int");
	return -1;
    }
    Py_INCREF(value);
    self->status_code = value;

    return 0;
}

static PyObject *NuclioResponse_getcontent_type(NuclioResponse *self,
						void *closure) {
    Py_INCREF(self->content_type);
    return self->content_type;
}

static int NuclioResponse_setcontent_type(NuclioResponse *self, PyObject *value,
					  void *closure) {
    if ((value == NULL) || (value == Py_None)) {
	Py_XDECREF(self->content_type);
	self->content_type = Py_BuildValue("");
	return 0;
    }

    if (!PyUnicode_Check(value)) {
	PyErr_SetString(PyExc_TypeError, "content_type must be a str");
	return -1;
    }
    Py_INCREF(value);
    self->content_type = value;

    return 0;
}

static PyObject *NuclioResponse_getheaders(NuclioResponse *self,
					   void *closure) {
    Py_INCREF(self->headers);
    return self->headers;
}

static int NuclioResponse_setheaders(NuclioResponse *self, PyObject *value,
				     void *closure) {
    if ((value == NULL) || (value == Py_None)) {
	Py_XDECREF(self->headers);
	self->headers = Py_BuildValue("");
	return 0;
    }

    if (!PyDict_Check(value)) {
	PyErr_SetString(PyExc_TypeError, "headers must be a dict");
	return -1;
    }
    Py_INCREF(value);
    self->headers = value;

    return 0;
}

static PyGetSetDef NuclioResponse_getsetlist[] = {
    {"body", (getter)NuclioResponse_getbody, (setter)NuclioResponse_setbody,
     NULL, "Response body"},
    {"status_code", (getter)NuclioResponse_getstatus_code,
     (setter)NuclioResponse_setstatus_code, NULL, "Response status code"},
    {"content_type", (getter)NuclioResponse_getcontent_type,
     (setter)NuclioResponse_setcontent_type, NULL, "Response content type"},
    {"headers", (getter)NuclioResponse_getheaders,
     (setter)NuclioResponse_setheaders, NULL, "Response headers"},
    {NULL}  // Sentinel
};

static int is_nothing(PyObject *obj) {
    if (obj == NULL) {
	return 1;
	return obj == Py_None;
    }
}

static int NuclioResponse_init(NuclioResponse *self, PyObject *args,
			       PyObject *kw) {
    PyObject *body = NULL, *status_code = NULL, *content_type = NULL,
	     *headers = NULL;
    static char *kwlist[] = {"body", "status_code", "content_type", "headers",
			     NULL};

    if (!PyArg_ParseTupleAndKeywords(args, kw, "|OOOO:Response", kwlist, &body,
				     &status_code, &content_type, &headers)) {
	return -1;
    }

    if (body == NULL) {
	body = PyUnicode_FromString("");
    }

    if (NuclioResponse_setbody(self, body, NULL) != 0) {
	return -1;
    }

    if (status_code == NULL) {
	status_code = PyLong_FromLong(200);
    }

    if (NuclioResponse_setstatus_code(self, status_code, NULL) != 0) {
	return -1;
    }

    if (content_type == NULL) {
	content_type = PyUnicode_FromString("text/plain");
    }

    if (NuclioResponse_setcontent_type(self, content_type, NULL) != 0) {
	return -1;
    }

    if (NuclioResponse_setheaders(self, headers, NULL) != 0) {
	return -1;
    }

    return 0;
}

static PyObject *NuclioResponse_repr(NuclioResponse *obj) {
    return PyUnicode_FromFormat(
	"Response(body=%R, status_code=%R, content_type=%R, headers=%R)",
	obj->body, obj->status_code, obj->content_type, obj->headers);
}

static PyTypeObject NuclioResponse_Type = {
    PyVarObject_HEAD_INIT(NULL, 0)

	"nuclio.Response",		      /* tp_name */
    sizeof(NuclioResponse),		      /* tp_basicsize */
    0,					      /* tp_itemsize */
    (destructor)NuclioResponse_dealloc,       /* tp_dealloc */
    0,					      /* tp_print */
    0,					      /* tp_getattr */
    0,					      /* tp_setattr */
    0,					      /* tp_reserved */
    (reprfunc)NuclioResponse_repr,	    /* tp_repr */
    0,					      /* tp_as_number */
    0,					      /* tp_as_sequence */
    0,					      /* tp_as_mapping */
    0,					      /* tp_hash  */
    0,					      /* tp_call */
    (reprfunc)NuclioResponse_repr,	    /* tp_str */
    0,					      /* tp_getattro */
    0,					      /* tp_setattro */
    0,					      /* tp_as_buffer */
    Py_TPFLAGS_DEFAULT | Py_TPFLAGS_BASETYPE, /* tp_flags */
    "Response objects",			      /* tp_doc */
    0,					      /* tp_traverse */
    0,					      /* tp_clear */
    0,					      /* tp_richcompare */
    0,					      /* tp_weaklistoffset */
    0,					      /* tp_iter */
    0,					      /* tp_iternext */
    0,					      /* tp_methods */
    0,					      /* tp_members */
    NuclioResponse_getsetlist,		      /* tp_getset */
    0,					      /* tp_base */
    0,					      /* tp_dict */
    0,					      /* tp_descr_get */
    0,					      /* tp_descr_set */
    0,					      /* tp_dictoffset */
    (initproc)NuclioResponse_init,	    /* tp_init */
    0,					      /* tp_alloc */
    NuclioResponse_new,			      /* tp_new */
};

int initialize_response_type(void) {
    if (PyType_Ready(&NuclioResponse_Type) == -1) {
	// TODO
	printf("ERROR: Response NOT READY");
	return 0;
    }

    Py_INCREF(&NuclioResponse_Type);

    return 1;
}

PyObject *response_type(void) { return (PyObject *)&NuclioResponse_Type; }

response_t as_response_t(PyObject *obj) {
    response_t response;

    if (PyObject_Type(obj) != response_type()) {
	PyErr_SetString(PyExc_TypeError, "Object is not nuclio.Response");
	return response;
    }

    NuclioResponse *robj = (NuclioResponse *)obj;
    response.body = robj->body;
    Py_INCREF(response.body);
    response.status_code = robj->status_code;
    Py_INCREF(response.status_code);
    response.content_type = robj->content_type;
    Py_INCREF(response.content_type);
    response.headers = robj->headers;
    Py_INCREF(response.headers);

    return response;
}


void free_response_t(response_t response) {
    Py_XDECREF(response.body);
    Py_XDECREF(response.status_code);
    Py_XDECREF(response.content_type);
    Py_XDECREF(response.headers);
}
