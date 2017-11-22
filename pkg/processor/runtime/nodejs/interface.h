#ifndef JS_H
#define JS_H

#ifdef __cplusplus
extern "C" {
#endif

typedef struct {
  void *worker;
  char *error_message;
} new_result_t;

void initialize();
new_result_t new_worker(char *code, char *handler_name);

#ifdef __cplusplus
}
#endif

#endif
