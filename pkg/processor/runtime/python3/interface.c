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
#include <datetime.h>

#include "types.h"

#include <stdlib.h>

static PyObject *handler_function = NULL;

extern int initialize_event_type(void);
extern PyObject *new_event(unsigned long);
extern int initialize_context_type(void);
extern PyObject *new_context(PyObject *);
extern int initialize_logger_type(void);
extern PyObject *new_logger(unsigned long);
extern int initialize_trigger_info_type(void);
extern void initialize_response_type(void);
extern PyObject *response_type(void);
extern response_t as_response_t(PyObject *);

void init_python(void) {
    Py_Initialize();
    PyDateTime_IMPORT;

    initialize_trigger_info_type();
    initialize_event_type();
    initialize_logger_type();
    initialize_context_type();
    initialize_response_type();
}

// Load hander function from module and save it in handler_function static
// variable
int load_handler(char *module_name, char *handler_name) {
    PyObject *module = PyImport_ImportModule(module_name);
    if (PyErr_Occurred()) {
	return 0;
    }

    PyObject *attr_name = PyUnicode_FromString(handler_name);
    handler_function = PyObject_GetAttr(module, attr_name);
    Py_DECREF(attr_name);

    if (PyErr_Occurred()) {
	return 0;
    }

    return 1;
}

static PyObject *response_from_output(PyObject *output) {
    PyObject *rtype = response_type();

    if (PyObject_IsInstance(output, rtype)) {
	return output;
    }

    PyObject *args = NULL;

    if (output == Py_None) {
	args = Py_BuildValue("()");
    } else if (PyUnicode_Check(output)) {
	args = Py_BuildValue("(O)", output);
    } else if (PyTuple_Check(output) && (PyObject_Length(output) == 2)) {
	PyObject *status_code = PyTuple_GetItem(output, 0);
	PyObject *body = PyTuple_GetItem(output, 1);

	args = Py_BuildValue("(OO)", body, status_code);
    } else {
	PyObject *type = PyObject_Type(output);
	PyErr_Format(PyExc_TypeError, "Unknown response type: %s (%s)", output,
		     type);
	Py_DECREF(type);
	return NULL;
    }

    PyObject *response = PyObject_CallObject(rtype, args);
    Py_DECREF(args);
    if (PyErr_Occurred()) {
	return NULL;
    }
    return response;
}

response_t call_handler(unsigned long event_ptr, unsigned long logger_ptr) {
    response_t response;
    
    if (!PyCallable_Check(handler_function)) {
	PyErr_SetString(PyExc_TypeError, "Handler if not a function");
	return response;
    }

    PyObject *event = new_event(event_ptr);
    if (event == NULL) {
	return response;
    }

    PyObject *logger = new_logger(logger_ptr);
    if (logger == NULL) {
	return response;
    }

    PyObject *context = new_context(logger);
    if (context == NULL) {
	return response;
    }

    PyObject *output =
	PyObject_CallFunctionObjArgs(handler_function, context, event, NULL);
    if (output == NULL) {
	return response;
    }

    PyObject *obj = response_from_output(output);
    return as_response_t(obj);
}

PyObject *new_datetime(int year, int month, int day, int hour, int minute,
		       int second, int usec) {
    // PyDateTime_FromDateAndTime is a macro, can be used by CGO
    return PyDateTime_FromDateAndTime(year, month, day, hour, minute, second,
				      usec);
}

// This is here since PyTYPE_Check are macros and we can't use them from cgo
int py_type(PyObject *obj) {
    if (PyUnicode_Check(obj)) {
	return PY_TYPE_UNICODE;
    }

    if (PyLong_Check(obj)) {
	return PY_TYPE_LONG;
    }

    if (PyFloat_Check(obj)) {
	return PY_TYPE_FLOAT;
    }

    return PY_TYPE_UNKNOWN;
}

char *py_obj_str(PyObject *obj) {
    PyObject *str = PyObject_Str(obj);
    char *val = PyUnicode_AsUTF8(str);

    Py_DECREF(str);

    return val;
}

char *py_type_name(PyObject *obj) {
    PyObject *obj_type = PyObject_Type(obj);

    char *type_name = py_obj_str(obj_type);

    Py_DECREF(obj_type);
    return type_name;
}

char *py_last_error() {
    PyObject *exc_type = PyErr_Occurred();
    if (exc_type == NULL) {
	return NULL;
    }

    // TODO: Traceback of an exception that has not been caught yet
    return py_obj_str(exc_type);
}

int py_is_none(PyObject *obj) {
    return obj == Py_None;
}
