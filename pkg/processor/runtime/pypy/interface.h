// Interface definitions, no Go types please

// FIXME: Doesn't work
typedef struct {
  char *body;
  char *content_type;
  long long status_code;
  // TODO: headers
  char *error;
} response_t;

struct API {
    char * (*handle_event)(void *);
    //response_t (*handle_event)(void *);
    char * (*set_handler)(char *);

    long long (*eventVersion)(void *);
    char* (*eventID)(void *);
    char* (*eventTriggerClass)(void *);
    char* (*eventTriggerKind)(void *);
    char* (*eventContentType)(void *);
    char* (*eventBody)(void *);
    long long (*eventSize)(void *ptr);
    char* (*eventHeader)(void *, char *);
    double (*eventTimestamp)(void *);
    char* (*eventPath)(void *);
    char* (*eventURL)(void *);
    char* (*eventMethod)(void *);

};


struct API api;

// exported from interface.go
extern long long eventVersion(void *);
extern char* eventID(void *);
extern long long eventSize(void *);
extern char* eventTriggerClass(void *);
extern char* eventTriggerKind(void *);
extern char* eventContentType(void *);
extern char* eventBody(void *);
extern char* eventHeader(void *, char *);
extern double eventTimestamp(void *);
extern char* eventPath(void *);
extern char* eventURL(void *);
extern char* eventMethod(void *);

// cgo can't call api functions directly
//response_t handle_event(void *ptr) {
char * handle_event(void *ptr) {
    api.handle_event(ptr);
}

char *set_handler(char *handler) {
    return api.set_handler(handler);
}

void init() {
    api.eventVersion = eventVersion;
    api.eventID = eventID;
    api.eventTriggerClass = eventTriggerClass;
    api.eventTriggerKind = eventTriggerKind;
    api.eventContentType = eventContentType;
    api.eventBody = eventBody;
    api.eventSize = eventSize;
    api.eventHeader = eventHeader;
    api.eventTimestamp = eventTimestamp;
    api.eventPath = eventPath;
    api.eventURL = eventURL;
    api.eventMethod = eventMethod;
}
