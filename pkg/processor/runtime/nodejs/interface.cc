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

// Based on samples/process.cc in V8 repo

#include <libplatform/libplatform.h>
#include <v8.h>

#include <iostream>
#include <sstream>

#include <stdlib.h>
#include <string.h>

#include "interface.h"
#include "log_levels.h"

namespace nuclio {
// Auto generated by Go
#include "_cgo_export.h"
}

using namespace v8;

void *unwrap_ptr(Local<Object> obj) {
  Local<External> field = Local<External>::Cast(obj->GetInternalField(0));
  return field->Value();
}

// Event methods

// Helper function to get all string methods
void getEventString(char *(func)(void *),
                    const PropertyCallbackInfo<Value> &info) {
  void *ptr = unwrap_ptr(info.Holder());
  char *value = func(ptr);
  info.GetReturnValue().Set(String::NewFromUtf8(info.GetIsolate(), value));
  // Go is dynamically allocating memory in C.CString
  free(value);
}

void GetEventVersion(Local<String> name,
                     const PropertyCallbackInfo<Value> &info) {
  void *ptr = unwrap_ptr(info.Holder());
  long long value = nuclio::eventVersion(ptr);
  return info.GetReturnValue().Set(Integer::New(info.GetIsolate(), value));
}

void GetEventID(Local<String> name, const PropertyCallbackInfo<Value> &info) {
  getEventString(nuclio::eventID, info);
}

void GetEventSize(Local<String> name, const PropertyCallbackInfo<Value> &info) {
  void *ptr = unwrap_ptr(info.Holder());
  long long value = nuclio::eventSize(ptr);
  return info.GetReturnValue().Set(Integer::New(info.GetIsolate(), value));
}

void GetEventTriggerClass(Local<String> name,
                          const PropertyCallbackInfo<Value> &info) {
  getEventString(nuclio::eventTriggerClass, info);
}

void GetEventTriggerKind(Local<String> name,
                         const PropertyCallbackInfo<Value> &info) {
  getEventString(nuclio::eventTriggerKind, info);
}

void GetEventContentType(Local<String> name,
                         const PropertyCallbackInfo<Value> &info) {
  getEventString(nuclio::eventContentType, info);
}

void GetEventBody(Local<String> name, const PropertyCallbackInfo<Value> &info) {
  getEventString(nuclio::eventBody, info);
}

void GetEventTimestamp(Local<String> name,
                       const PropertyCallbackInfo<Value> &info) {
  void *ptr = unwrap_ptr(info.Holder());
  double value = nuclio::eventTimestamp(ptr);
  info.GetReturnValue().Set(Date::New(info.GetIsolate(), value));
}

void GetEventPath(Local<String> name, const PropertyCallbackInfo<Value> &info) {
  getEventString(nuclio::eventPath, info);
}

void GetEventURL(Local<String> name, const PropertyCallbackInfo<Value> &info) {
  getEventString(nuclio::eventURL, info);
}

void GetEventMethod(Local<String> name,
                    const PropertyCallbackInfo<Value> &info) {
  getEventString(nuclio::eventMethod, info);
}

void getEventMap(char *(func)(void *),
                 const PropertyCallbackInfo<Value> &info) {
  void *ptr = unwrap_ptr(info.Holder());
  char *value = func(ptr);

  Local<String> json = String::NewFromUtf8(info.GetIsolate(), value);
  Local<Value> parsed = JSON::Parse(info.GetIsolate(), json).ToLocalChecked();
  Local<Object> headers = Local<Object>::Cast(parsed);
  info.GetReturnValue().Set(headers);
}

void GetEventHeaders(Local<String> name,
                     const PropertyCallbackInfo<Value> &info) {
  getEventMap(nuclio::eventHeaders, info);
}

void GetEventFields(Local<String> name,
                    const PropertyCallbackInfo<Value> &info) {
  getEventMap(nuclio::eventFields, info);
}

void contextLog(const FunctionCallbackInfo<Value> &args, int level) {
  if (args.Length() < 1) {
    // TODO: Raise exception in JS
    return;
  }

  void *ptr = unwrap_ptr(args.Holder());

  Isolate *isolate = args.GetIsolate();
  HandleScope scope(isolate);
  Local<Value> arg = args[0];
  String::Utf8Value message(isolate, arg);

  nuclio::contextLog(ptr, level, *message);
}

void ContextLogError(const FunctionCallbackInfo<Value> &args) {
  contextLog(args, LOG_LEVEL_ERROR);
}

void ContextLogWarning(const FunctionCallbackInfo<Value> &args) {
  contextLog(args, LOG_LEVEL_WARNING);
}

void ContextLogInfo(const FunctionCallbackInfo<v8::Value> &args) {
  contextLog(args, LOG_LEVEL_INFO);
}

void ContextLogDebug(const FunctionCallbackInfo<v8::Value> &args) {
  contextLog(args, LOG_LEVEL_DEBUG);
}

void contextLogWith(const FunctionCallbackInfo<v8::Value> &args, int level) {
  if (args.Length() < 2) {
    // TODO: Raise exception in JS
    return;
  }

  void *ptr = unwrap_ptr(args.Holder());

  Isolate *isolate = args.GetIsolate();
  HandleScope scope(isolate);
  Local<Value> arg0 = args[0];
  String::Utf8Value format(isolate, arg0);

  Local<Object> with = Local<Object>::Cast(args[1]);
  MaybeLocal<String> maybe_json =
      JSON::Stringify(isolate->GetCurrentContext(), with);
  if (maybe_json.IsEmpty()) {
    maybe_json = String::NewFromUtf8(isolate, "{}", NewStringType::kNormal, 2);
  }
  String::Utf8Value json(isolate, maybe_json.ToLocalChecked());
  nuclio::contextLogWith(ptr, level, *format, *json);
}

void ContextLogErrorWith(const FunctionCallbackInfo<v8::Value> &args) {
  contextLogWith(args, LOG_LEVEL_ERROR);
}

void ContextLogWarningWith(const FunctionCallbackInfo<v8::Value> &args) {
  contextLogWith(args, LOG_LEVEL_WARNING);
}

void ContextLogInfoWith(const FunctionCallbackInfo<v8::Value> &args) {
  contextLogWith(args, LOG_LEVEL_INFO);
}

void ContextLogDebugWith(const FunctionCallbackInfo<v8::Value> &args) {
  contextLogWith(args, LOG_LEVEL_DEBUG);
}

class JSWorker {
public:
  JSWorker(Isolate *isolate, Local<String> script, Local<String> handler_name)
      : isolate_(isolate), script_(script), handler_name_(handler_name) {}

  virtual ~JSWorker() {
    context_.Reset();
    handler_.Reset();
  }

  char *initialize() {
    char *error;

    HandleScope handle_scope(isolate_);
    Local<ObjectTemplate> global = ObjectTemplate::New(isolate_);

    // Each handler gets its own context so different handler don't affect each
    // other. Context::New returns a persistent handle which is what we need
    // for the reference to remain after we return from this method. That
    // persistent handle has to be disposed in the destructor.
    Local<Context> context = Context::New(isolate_, NULL, global);
    context_.Reset(isolate_, context);

    // Enter the new context so all the following operations take place
    // within it.
    Context::Scope context_scope(context);

    // Compile and run the script
    error = load_script(script_);
    if (error != NULL) {
      return error;
    }

    make_event_template();
    make_context_template();

    // All done; all went well
    return NULL;
  }

  response_t handle_event(void *nuclio_context, void *nuclio_event) {
    response_t response = {
        NULL, // headers
        NULL, // body
        NULL, // content_type
        0,    // status_code
        NULL  // error_message
    };

    // TODO: Maybe use uv_event loop?
    Locker locker(isolate_);
    HandleScope handle_scope(isolate_);
    Local<Context> context = Local<Context>::New(isolate_, context_);
    Context::Scope context_scope(context);

    // Invoke the handler function, giving the global object as 'this'
    // and one argument, the request.
    Local<Object> event = wrap_event(nuclio_event);
    Local<Object> ctx = wrap_context(nuclio_context);
    int argc = 2;
    Local<Value> argv[argc] = {ctx, event};
    Local<v8::Function> handler = Local<v8::Function>::New(isolate_, handler_);

    TryCatch try_catch(isolate_);
    try_catch.SetVerbose(true); // Get errors in stdout

    MaybeLocal<Value> maybe_result =
        handler->Call(context, context->Global(), argc, argv);

    if (try_catch.HasCaught()) {
      String::Utf8Value error(isolate_, try_catch.Exception());
      response.error_message = strdup(*error);
      return response;
    }

    Local<Value> result;
    if (!maybe_result.ToLocal(&result)) {
      response.error_message = strdup("Empty result");
      return response;
    }

    if (result->IsString()) {
      String::Utf8Value result_str(isolate_, result);
      response.body = strdup(*result_str);
      response.content_type = strdup("text/plain");
      response.status_code = 200;
    } else if (result->IsArray()) {
      parseArrayResult(result, &response);
    } else if (result->IsObject()) {
      parseObjectResult(result, &response);
    } else {
      Local<String> type = result->TypeOf(isolate_);

      std::ostringstream oss;
      oss << "Unkwnown result type " << *type;
      response.error_message = strdup(oss.str().c_str());
    }

    if ((response.error_message == NULL) && (response.body == NULL)) {
      response.body = jsonify(result);
      if (response.body == NULL) {
        response.error_message = strdup("Can't jsonify result");
      } else {
        response.content_type = strdup("application/json");
      }
    }

    return response;
  }

private:
  char *jsonify(Local<Value> value) {
    Local<Object> object = Local<Object>::Cast(value);
    MaybeLocal<String> maybe_json =
        JSON::Stringify(isolate_->GetCurrentContext(), object);
    if (maybe_json.IsEmpty()) {
      return NULL;
    }

    String::Utf8Value json(isolate_, maybe_json.ToLocalChecked());
    return strdup(*json);
  }

  void parseArrayResult(Local<Value> result, response_t *response) {
    Handle<Array> array = Handle<Array>::Cast(result);
    if (array->Length() != 2) { // Should be status, body
      return;
    }

    Local<Value> status;
    Local<Value> i0 = Integer::New(isolate_, 0);
    if (!array->Get(isolate_->GetCurrentContext(), i0).ToLocal(&status)) {
      response->error_message = strdup("Can't get element 0 from result");
      return;
    }
    response->status_code = Local<Integer>::Cast(status)->Value();
    if (response->status_code == 0) { // Not a number
      return;
    }

    Local<Value> body_value;
    Local<Value> i1 = Integer::New(isolate_, 1);
    if (!array->Get(isolate_->GetCurrentContext(), i1).ToLocal(&body_value)) {
      response->error_message = strdup("Can't get element 1 from result");
      response->status_code = 0;
      return;
    }

    if (body_value->IsString()) {
      String::Utf8Value body(isolate_, body_value);
      response->body = strdup(*body);
    } else {
      response->body = jsonify(body_value);
      if (response->body == NULL) {
        response->error_message = strdup("Can't convert body to JSON");
        return;
      }
      response->content_type = strdup("application/json");
    }
  }

  void parseObjectResult(Local<Value> result, response_t *response) {
    Local<Object> object = Local<Object>::Cast(result);
    Local<Value> body = object->Get(String::NewFromUtf8(isolate_, "body"));

    if (body->IsString()) {
      String::Utf8Value body_str(isolate_, body);
      response->body = strdup(*body_str);
    } else {
      response->body = jsonify(body);
      if (response->body == NULL) {
        response->error_message = strdup("Can't encode body");
        return;
      }
      response->content_type = strdup("application/json");
    }

    Local<Value> content_type =
        object->Get(String::NewFromUtf8(isolate_, "content_type"));
    if (content_type->IsString()) {
      String::Utf8Value ctype_str(isolate_, content_type);
      response->content_type = strdup(*ctype_str);
    } else if ((content_type->IsUndefined()) || (content_type->IsNull())) {
      // NOP
    } else {
      response->error_message = strdup("content_type is not a string");
      return;
    }

    Local<Value> status_code =
        object->Get(String::NewFromUtf8(isolate_, "status_code"));
    if (!status_code->IsNumber()) {
      response->error_message = strdup("status_code is not a number");
      return;
    }
    response->status_code = Local<Integer>::Cast(status_code)->Value();

    Local<Value> headers =
        object->Get(String::NewFromUtf8(isolate_, "headers"));
    if (!((headers->IsUndefined()) || (headers->IsNull()))) {
      response->headers = jsonify(headers);
      if (response->headers == NULL) {
        response->error_message = strdup("Can't convert headers to JSON");
        return;
      }
    }
  }

  char *load_script(Local<String> code) {
    Locker locker(isolate_);
    /*
    Isolate::Scope isolate_scope(isolate_);
    HandleScope handle_scope(isolate_);
    */

    Local<Context> context = Local<Context>::New(isolate_, context_);
    // Context::Scope context_scope(context);

    TryCatch try_catch;

    ScriptOrigin origin(String::NewFromUtf8(isolate_, "handler.js"));
    Local<Script> script = Script::Compile(code, &origin);

    if (script.IsEmpty()) {
      return exception_string(isolate_, &try_catch);
    }

    Handle<Value> result = script->Run();

    if (result.IsEmpty()) {
      return exception_string(isolate_, &try_catch);
    }

    Local<Value> handler;
    if (!context->Global()->Get(context, handler_name_).ToLocal(&handler)) {
      String::Utf8Value handler_name(isolate_, handler_name_);
      std::ostringstream oss;
      oss << "Can't find " << *handler_name << " in code";
      return strdup(oss.str().c_str());
    }

    if (!handler->IsFunction()) {
      std::ostringstream oss;
      String::Utf8Value handler_name(isolate_, handler_name_);
      oss << *handler_name << " is not a function";
      return strdup(oss.str().c_str());
    }

    handler_.Reset(isolate_, Local<Function>::Cast(handler));
    return NULL;
  }

  char *exception_string(Isolate *isolate, TryCatch *try_catch) {

    std::ostringstream oss;
    HandleScope handle_scope(isolate);
    String::Utf8Value exception(try_catch->Exception());
    Handle<Message> message = try_catch->Message();

    if (message.IsEmpty()) {
      oss << *exception << "\n";
    } else {
      int lineno = message->GetLineNumber();

      oss << lineno << ": " << *exception << "\n";
      String::Utf8Value source_line(message->GetSourceLine());
      oss << *source_line << "\n";

      // Print ^^^
      int start = message->GetStartColumn();
      for (int i = 0; i < start; i++) {
        oss << " ";
      }
      int end = message->GetEndColumn();
      for (int i = start; i < end; i++) {
        oss << "^";
      }
      oss << "\n";
      String::Utf8Value stack_trace(try_catch->StackTrace());
      if (stack_trace.length() > 0) {
        oss << *stack_trace << "\n";
      }
    }

    return strdup(oss.str().c_str());
  }

  Local<Object> wrap_event(void *ptr) {
    Local<ObjectTemplate> templ =
        Local<ObjectTemplate>::New(isolate_, event_template_);

    Local<Object> result =
        templ->NewInstance(isolate_->GetCurrentContext()).ToLocalChecked();
    Local<External> event_ptr = External::New(isolate_, ptr);
    result->SetInternalField(0, event_ptr);
    return result;
  }

  Local<String> intern(const char *value) {
    return String::NewFromUtf8(isolate_, value, NewStringType::kInternalized)
        .ToLocalChecked();
  }

  void make_event_template() {
    EscapableHandleScope handle_scope(isolate_);

    Local<ObjectTemplate> result = ObjectTemplate::New(isolate_);
    result->SetInternalFieldCount(1);

    // Add accessors for each of the event fields
    result->SetAccessor(intern("version"), GetEventVersion);
    result->SetAccessor(intern("id"), GetEventID);
    result->SetAccessor(intern("size"), GetEventSize);
    result->SetAccessor(intern("trigger_class"), GetEventTriggerClass);
    result->SetAccessor(intern("trigger_kind"), GetEventTriggerKind);
    result->SetAccessor(intern("content_type"), GetEventContentType);
    result->SetAccessor(intern("body"), GetEventBody);
    result->SetAccessor(intern("timestamp"), GetEventTimestamp);
    result->SetAccessor(intern("path"), GetEventPath);
    result->SetAccessor(intern("url"), GetEventURL);
    result->SetAccessor(intern("method"), GetEventMethod);
    result->SetAccessor(intern("headers"), GetEventHeaders);
    result->SetAccessor(intern("fields"), GetEventFields);

    event_template_.Reset(isolate_, result);
  }

  Local<Object> wrap_context(void *ptr) {
    Local<ObjectTemplate> templ =
        Local<ObjectTemplate>::New(isolate_, context_template_);

    Local<Object> result =
        templ->NewInstance(isolate_->GetCurrentContext()).ToLocalChecked();
    Local<External> context_ptr = External::New(isolate_, ptr);
    result->SetInternalField(0, context_ptr);
    return result;
  }

  void make_context_template() {
    Local<ObjectTemplate> result = ObjectTemplate::New(isolate_);
    result->SetInternalFieldCount(1);

    // Add methods for each of the logging functions
    result->Set(intern("log_error"),
                FunctionTemplate::New(isolate_, ContextLogError));
    result->Set(intern("log_warn"),
                FunctionTemplate::New(isolate_, ContextLogWarning));
    result->Set(intern("log_info"),
                FunctionTemplate::New(isolate_, ContextLogInfo));
    result->Set(intern("log_debug"),
                FunctionTemplate::New(isolate_, ContextLogDebug));
    result->Set(intern("log_error_with"),
                FunctionTemplate::New(isolate_, ContextLogErrorWith));
    result->Set(intern("log_warn_with"),
                FunctionTemplate::New(isolate_, ContextLogWarningWith));
    result->Set(intern("log_info_with"),
                FunctionTemplate::New(isolate_, ContextLogInfoWith));
    result->Set(intern("log_debug_with"),
                FunctionTemplate::New(isolate_, ContextLogDebugWith));

    context_template_.Reset(isolate_, result);
  }

  Isolate *isolate_;
  Persistent<Context> context_;
  Local<String> script_;
  Local<String> handler_name_;
  Persistent<Function> handler_;
  Persistent<ObjectTemplate> event_template_;
  Persistent<ObjectTemplate> context_template_;
};

static bool initialized_(false);

extern "C" {

void initialize() {
  if (initialized_) {
    return;
  }

  V8::InitializeICUDefaultLocation("nuclio");
  V8::InitializeExternalStartupData("nuclio");
  Platform *platform = v8::platform::CreateDefaultPlatform();
  V8::InitializePlatform(platform);
  V8::Initialize();

  initialized_ = true;
  // TODO: Inject nodes's require
}

new_result_t new_worker(char *code, char *handler_name) {
  new_result_t result = {NULL, NULL};

  Isolate::CreateParams create_params;
  create_params.array_buffer_allocator =
      ArrayBuffer::Allocator::NewDefaultAllocator();
  Isolate *isolate = Isolate::New(create_params);
  isolate->SetCaptureStackTraceForUncaughtExceptions(true);
  Locker locker(isolate);
  Isolate::Scope isolate_scope(isolate);
  HandleScope handle_scope(isolate);

  Local<String> source = String::NewFromUtf8(isolate, code);
  Local<String> handler = String::NewFromUtf8(isolate, handler_name);

  JSWorker *worker = new JSWorker(isolate, source, handler);
  char *error = worker->initialize();
  if (error != NULL) {
    delete worker;
    worker = NULL;
    result.error_message = error;
    return result;
  }

  result.worker = worker;
  return result;
}

response_t handle_event(void *worker, void *context, void *event) {
  JSWorker *jsworker = (JSWorker *)worker;

  return jsworker->handle_event(context, event);
}

void free_response(response_t response) {
  if (response.headers != NULL) {
    free(response.headers);
  }

  if (response.body != NULL) {
    free(response.body);
  }

  if (response.content_type != NULL) {
    free(response.content_type);
  }

  if (response.error_message != NULL) {
    free(response.error_message);
  }
}

} // extern "C"
