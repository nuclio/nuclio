// +build pypy

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
#ifndef INTERFACE_H
#define INTERFACE_H

#include "types.h"

struct API {
  response_t *(*handle_event)(void *context, void *event);
  char *(*set_handler)(char *);

  char *(*eventID)(void *);
  char *(*eventTriggerClass)(void *);
  char *(*eventTriggerKind)(void *);
  char *(*eventContentType)(void *);
  bytes_t (*eventBody)(void *);
  long long (*eventSize)(void *ptr);
  char *(*eventHeaders)(void *);
  char *(*eventFields)(void *);
  double (*eventTimestamp)(void *);
  char *(*eventPath)(void *);
  char *(*eventURL)(void *);
  char *(*eventMethod)(void *);
  char *(*eventType)(void *);
  char *(*eventTypeVersion)(void *);
  char *(*eventVersion)(void *);

  void (*contextLog)(void *, int, char *);
  void (*contextLogWith)(void *, int, char *, char *);
};

struct API api;

// exported from interface.go
extern char *eventID(void *);
extern long long eventSize(void *);
extern char *eventTriggerClass(void *);
extern char *eventTriggerKind(void *);
extern char *eventContentType(void *);
extern bytes_t eventBody(void *);
extern char *eventHeaders(void *);
extern char *eventFields(void *);
extern double eventTimestamp(void *);
extern char *eventPath(void *);
extern char *eventURL(void *);
extern char *eventMethod(void *);
extern char *eventType(void *);
extern char *eventTypeVersion(void *);
extern char *eventVersion(void *);

extern void contextLog(void *, int, char *);
extern void contextLogWith(void *, int, char *, char *);

// cgo can't call api functions directly
response_t *handle_event(void *context, void *event) {
  return api.handle_event(context, event);
}

char *set_handler(char *handler) { return api.set_handler(handler); }

void fill_api() {
  api.eventID = eventID;
  api.eventTriggerClass = eventTriggerClass;
  api.eventTriggerKind = eventTriggerKind;
  api.eventContentType = eventContentType;
  api.eventBody = eventBody;
  api.eventSize = eventSize;
  api.eventHeaders = eventHeaders;
  api.eventFields = eventFields;
  api.eventTimestamp = eventTimestamp;
  api.eventPath = eventPath;
  api.eventURL = eventURL;
  api.eventMethod = eventMethod;
  api.eventType = eventType;
  api.eventTypeVersion = eventTypeVersion;
  api.eventVersion = eventVersion;

  api.contextLog = contextLog;
  api.contextLogWith = contextLogWith;
}

#endif  // #ifdef INTERFACE_H
