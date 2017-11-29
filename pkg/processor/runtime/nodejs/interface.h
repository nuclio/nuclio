// +build nodejs

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

// Interface definitions, no Go types please
//
#ifdef __cplusplus
extern "C" {
#endif

typedef struct {
  void *worker;
  char *error_message;
} new_result_t;

typedef struct {
  char *headers;
  char *body;
  char *content_type;
  int status_code;
  char *error_message;
} response_t;

void free_response(response_t response);

void initialize();
new_result_t new_worker(char *code, char *handler_name);
response_t handle_event(void *worker, void *context, void *event);

#ifdef __cplusplus
}
#endif
