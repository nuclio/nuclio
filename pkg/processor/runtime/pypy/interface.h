// Interface definitions, no Go types please

typedef struct {
  char *body;
  char *content_type;
  long long status_code;
  char *headers;
  char *error;
} response_t;

enum { LOG_LEVEL_ERROR, LOG_LEVEL_WARNING, LOG_LEVEL_INFO, LOG_LEVEL_DEBUG };

struct API {
  response_t *(*handle_event)(void *context, void *event);
  char *(*set_handler)(char *);

  long long (*eventVersion)(void *);
  char *(*eventID)(void *);
  char *(*eventTriggerClass)(void *);
  char *(*eventTriggerKind)(void *);
  char *(*eventContentType)(void *);
  char *(*eventBody)(void *);
  long long (*eventSize)(void *ptr);
  char *(*eventHeaders)(void *);
  char *(*eventFields)(void *);
  double (*eventTimestamp)(void *);
  char *(*eventPath)(void *);
  char *(*eventURL)(void *);
  char *(*eventMethod)(void *);

  void (*contextLog)(void *, int, char *);
  void (*contextLogWith)(void *, int, char *, char *);
};

struct API api;

// exported from interface.go
extern long long eventVersion(void *);
extern char *eventID(void *);
extern long long eventSize(void *);
extern char *eventTriggerClass(void *);
extern char *eventTriggerKind(void *);
extern char *eventContentType(void *);
extern char *eventBody(void *);
extern char *eventHeaders(void *);
extern char *eventFields(void *);
extern double eventTimestamp(void *);
extern char *eventPath(void *);
extern char *eventURL(void *);
extern char *eventMethod(void *);

extern void contextLog(void *, int, char *);
extern void contextLogWith(void *, int, char *, char *);

// cgo can't call api functions directly
response_t *handle_event(void *context, void *event) {
  return api.handle_event(context, event);
}

char *set_handler(char *handler) { return api.set_handler(handler); }

void fill_api() {
  api.eventVersion = eventVersion;
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

  api.contextLog = contextLog;
  api.contextLogWith = contextLogWith;
}
