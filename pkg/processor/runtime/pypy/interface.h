// Interface definitions, no Go types please

typedef struct {
  char *body;
  char *content_type;
  long long status_code;
  // TODO: headers
  char *error;
} response_t;

struct API {
    response_t* (*handle_event)(void *context, void *event);
    char* (*set_handler)(char *);

    long long (*eventVersion)(void *);
    char* (*eventID)(void *);
    char* (*eventTriggerClass)(void *);
    char* (*eventTriggerKind)(void *);
    char* (*eventContentType)(void *);
    char* (*eventBody)(void *);
    long long (*eventSize)(void *ptr);
    char* (*eventHeaderString)(void *, char *);
    char* (*eventFieldString)(void *, char *);
    double (*eventTimestamp)(void *);
    char* (*eventPath)(void *);
    char* (*eventURL)(void *);
    char* (*eventMethod)(void *);

    void (*contextLogError)(void *, char *);
    void (*contextLogWarn)(void *, char *);
    void (*contextLogInfo)(void *, char *);
    void (*contextLogDebug)(void *, char *);

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
extern char* eventHeaderString(void *, char *);
extern char* eventFieldString(void *, char *);
extern double eventTimestamp(void *);
extern char* eventPath(void *);
extern char* eventURL(void *);
extern char* eventMethod(void *);

extern void contextLogError(void *, char *);
extern void contextLogWarn(void *, char *);
extern void contextLogInfo(void *, char *);
extern void contextLogDebug(void *, char *);

// cgo can't call api functions directly
response_t *handle_event(void *context, void *event) {
    return api.handle_event(context, event);
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
    api.eventHeaderString = eventHeaderString;
    api.eventTimestamp = eventTimestamp;
    api.eventPath = eventPath;
    api.eventURL = eventURL;
    api.eventMethod = eventMethod;

    api.contextLogError = contextLogError;
    api.contextLogWarn = contextLogWarn;
    api.contextLogInfo = contextLogInfo;
    api.contextLogDebug = contextLogDebug;
}
