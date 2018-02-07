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

#ifndef TYPES_H
#define TYPES_H

#include <Python.h>

enum { LOG_LEVEL_ERROR, LOG_LEVEL_WARNING, LOG_LEVEL_INFO, LOG_LEVEL_DEBUG };
enum { PY_TYPE_UNKNOWN, PY_TYPE_UNICODE, PY_TYPE_LONG, PY_TYPE_FLOAT };

typedef struct {
  PyObject *body;
  PyObject *status_code;
  PyObject *content_type;
  PyObject *headers;
} response_t;

#endif
