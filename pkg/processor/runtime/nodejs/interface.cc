// Based on samples/process.cc in V8 repo
//
#include <libplatform/libplatform.h>
#include <v8.h>

#include <map>
#include <string>
#include <sstream>

#include <string.h>
/*
#include <stdlib.h>
*/

#include "interface.h"

using std::map;
using std::pair;
using std::string;

using v8::Context;
using v8::EscapableHandleScope;
using v8::External;
using v8::Function;
using v8::FunctionTemplate;
using v8::Global;
using v8::HandleScope;
using v8::Isolate;
using v8::Local;
using v8::MaybeLocal;
using v8::Name;
using v8::NamedPropertyHandlerConfiguration;
using v8::NewStringType;
using v8::Object;
using v8::ObjectTemplate;
using v8::PropertyCallbackInfo;
using v8::Script;
using v8::String;
using v8::TryCatch;
using v8::Value;

typedef struct {} HttpRequest;

class JSWorker{
public:
  JSWorker(Isolate *isolate, Local<String> script, Local<String> handler_name):
    isolate_(isolate), script_(script), handler_(handler_name) {
  }

  virtual ~JSWorker(){
    context_.Reset();
    process_.Reset();
  }

  char * Initialize() {
    char *error;

    // map<string, string> *opts, map<string, string> *output) {
    HandleScope handle_scope(isolate_);
    Local<ObjectTemplate> global = ObjectTemplate::New(isolate_);

    // Each processor gets its own context so different processors don't
    // affect each other. Context::New returns a persistent handle which
    // is what we need for the reference to remain after we return from
    // this method. That persistent handle has to be disposed in the
    // destructor.
    v8::Local<v8::Context> context = Context::New(isolate_, NULL, global);
    context_.Reset(isolate_, context);

    // Enter the new context so all the following operations take place
    // within it.
    Context::Scope context_scope(context);

    // Compile and run the script
    error = ExecuteScript(script_);
    if (error != NULL) {
      return error;
    }

    Local<Value> process_val;
    // If there is no handler function, or if it is not a function, bail out
    if (!context->Global()->Get(context, handler_).ToLocal(&process_val)) {
      String::Utf8Value handler_name(isolate_, handler_);
      std::ostringstream oss;
      oss << "Can't find " << *handler_name << " in code";
      return strdup(oss.str().c_str());
    }

    if (!process_val->IsFunction()) {
      std::ostringstream oss;
      String::Utf8Value handler_name(isolate_, handler_);
      oss << *handler_name << " is not a function";
      return strdup(oss.str().c_str());
    }

    // It is a function; cast it to a Function
    Local<Function> process_fun = Local<Function>::Cast(process_val);

    // Store the function in a Global handle, since we also want
    // that to remain after this call returns
    process_.Reset(isolate_, process_fun);

    // All done; all went well
    return NULL;
}

private:

  char * ExecuteScript(Local<String> script) {
    HandleScope handle_scope(isolate_);

    // We're just about to compile the script; set up an error handler to
    // catch any exceptions the script might throw.
    TryCatch try_catch(isolate_);

    Local<Context> context(isolate_->GetCurrentContext());

    // Compile the script and check for errors.
    Local<Script> compiled_script;
    if (!Script::Compile(context, script).ToLocal(&compiled_script)) {
      String::Utf8Value error(isolate_, try_catch.Exception());
      return strdup(*error);
    }

    // Run the script!
    Local<Value> result;
    if (!compiled_script->Run(context).ToLocal(&result)) {
      // The TryCatch above is still in effect and will have caught the error.
      String::Utf8Value error(isolate_, try_catch.Exception());
      return strdup(*error);
    }

    return NULL;
  }


/*
  Isolate *isolate_ { return isolate_; }
  virtual bool Initialize(map<string, string> *opts,
                          map<string, string> *output);
  virtual bool Process(HttpRequest *req);

private:
  // Execute the script associated with this processor and extract the
  // Process function.  Returns true if this succeeded, otherwise false.

  // Wrap the options and output map in a JavaScript objects and
  // install it in the global namespace as 'options' and 'output'.
  bool InstallMaps(map<string, string> *opts, map<string, string> *output);

  // Constructs the template that describes the JavaScript wrapper
  // type for requests.
  static Local<ObjectTemplate> MakeRequestTemplate(Isolate *isolate);
  static Local<ObjectTemplate> MakeMapTemplate(Isolate *isolate);

  // Callbacks that access the individual fields of request objects.
  static void GetPath(Local<String> name,
                      const PropertyCallbackInfo<Value> &info);
  static void GetReferrer(Local<String> name,
                          const PropertyCallbackInfo<Value> &info);
  static void GetHost(Local<String> name,
                      const PropertyCallbackInfo<Value> &info);
  static void GetUserAgent(Local<String> name,
                           const PropertyCallbackInfo<Value> &info);

  // Callbacks that access maps
  static void MapGet(Local<Name> name, const PropertyCallbackInfo<Value> &info);
  static void MapSet(Local<Name> name, Local<Value> value,
                     const PropertyCallbackInfo<Value> &info);

  // Utility methods for wrapping C++ objects as JavaScript objects,
  // and going back again.
  Local<Object> WrapMap(map<string, string> *obj);
  static map<string, string> *UnwrapMap(Local<Object> obj);
  Local<Object> WrapRequest(HttpRequest *obj);
  static HttpRequest *UnwrapRequest(Local<Object> obj);

  */


  Isolate *isolate_;
  Local<String> script_;
  Local<String> handler_;
  Global<Context> context_;
  Global<Function> process_;
  static Global<ObjectTemplate> request_template_;
  static Global<ObjectTemplate> map_template_;
};

/*
static void LogCallback(const v8::FunctionCallbackInfo<v8::Value> &args) {
  if (args.Length() < 1)
    return;
  Isolate *isolate = args.isolate_;
  HandleScope scope(isolate);
  Local<Value> arg = args[0];
  String::Utf8Value value(isolate, arg);
  HttpRequestProcessor::Log(*value);
}



bool JSWorker::InstallMaps(map<string, string> *opts,
                                         map<string, string> *output) {
  HandleScope handle_scope(isolate_);

  // Wrap the map object in a JavaScript wrapper
  Local<Object> opts_obj = WrapMap(opts);

  v8::Local<v8::Context> context =
      v8::Local<v8::Context>::New(isolate_, context_);

  // Set the options object as a property on the global object.
  context->Global()
      ->Set(context,
            String::NewFromUtf8(isolate_, "options", NewStringType::kNormal)
                .ToLocalChecked(),
            opts_obj)
      .FromJust();

  Local<Object> output_obj = WrapMap(output);
  context->Global()
      ->Set(context,
            String::NewFromUtf8(isolate_, "output", NewStringType::kNormal)
                .ToLocalChecked(),
            output_obj)
      .FromJust();

  return true;
}

bool JSWorker::Process(HttpRequest *request) {
  // Create a handle scope to keep the temporary object references.
  HandleScope handle_scope(isolate_);

  v8::Local<v8::Context> context =
      v8::Local<v8::Context>::New(isolate_, context_);

  // Enter this processor's context so all the remaining operations
  // take place there
  Context::Scope context_scope(context);

  // Wrap the C++ request object in a JavaScript wrapper
  Local<Object> request_obj = WrapRequest(request);

  // Set up an exception handler before calling the Process function
  TryCatch try_catch(isolate_);

  // Invoke the process function, giving the global object as 'this'
  // and one argument, the request.
  const int argc = 1;
  Local<Value> argv[argc] = {request_obj};
  v8::Local<v8::Function> process =
      v8::Local<v8::Function>::New(isolate_, process_);
  Local<Value> result;
  if (!process->Call(context, context->Global(), argc, argv).ToLocal(&result)) {
    String::Utf8Value error(isolate_, try_catch.Exception());
    Log(*error);
    return false;
  }
  return true;
}
*/



/*
 
Global<ObjectTemplate> JSWorker::request_template_;
Global<ObjectTemplate> JSWorker::map_template_;

// Utility function that wraps a C++ http request object in a
// JavaScript object.
Local<Object> JSWorker::WrapMap(map<string, string> *obj) {
  // Local scope for temporary handles.
  EscapableHandleScope handle_scope(isolate_);

  // Fetch the template for creating JavaScript map wrappers.
  // It only has to be created once, which we do on demand.
  if (map_template_.IsEmpty()) {
    Local<ObjectTemplate> raw_template = MakeMapTemplate(isolate_);
    map_template_.Reset(isolate_, raw_template);
  }
  Local<ObjectTemplate> templ =
      Local<ObjectTemplate>::New(isolate_, map_template_);

  // Create an empty map wrapper.
  Local<Object> result =
      templ->NewInstance(isolate_->GetCurrentContext()).ToLocalChecked();

  // Wrap the raw C++ pointer in an External so it can be referenced
  // from within JavaScript.
  Local<External> map_ptr = External::New(isolate_, obj);

  // Store the map pointer in the JavaScript wrapper.
  result->SetInternalField(0, map_ptr);

  // Return the result through the current handle scope.  Since each
  // of these handles will go away when the handle scope is deleted
  // we need to call Close to let one, the result, escape into the
  // outer handle scope.
  return handle_scope.Escape(result);
}

// Utility function that extracts the C++ map pointer from a wrapper
// object.
map<string, string> *JSWorker::UnwrapMap(Local<Object> obj) {
  Local<External> field = Local<External>::Cast(obj->GetInternalField(0));
  void *ptr = field->Value();
  return static_cast<map<string, string> *>(ptr);
}

// Convert a JavaScript string to a std::string.  To not bother too
// much with string encodings we just use ascii.
string ObjectToString(v8::Isolate *isolate, Local<Value> value) {
  String::Utf8Value utf8_value(isolate, value);
  return string(*utf8_value);
}

void JSWorker::MapGet(Local<Name> name,
                                    const PropertyCallbackInfo<Value> &info) {
  if (name->IsSymbol())
    return;

  // Fetch the map wrapped by this object.
  map<string, string> *obj = UnwrapMap(info.Holder());

  // Convert the JavaScript string to a std::string.
  string key = ObjectToString(info.isolate_, Local<String>::Cast(name));

  // Look up the value if it exists using the standard STL ideom.
  map<string, string>::iterator iter = obj->find(key);

  // If the key is not present return an empty handle as signal
  if (iter == obj->end())
    return;

  // Otherwise fetch the value and wrap it in a JavaScript string
  const string &value = (*iter).second;
  info.GetReturnValue().Set(
      String::NewFromUtf8(info.isolate_, value.c_str(),
                          NewStringType::kNormal,
                          static_cast<int>(value.length()))
          .ToLocalChecked());
}

void JSWorker::MapSet(Local<Name> name, Local<Value> value_obj,
                                    const PropertyCallbackInfo<Value> &info) {
  if (name->IsSymbol())
    return;

  // Fetch the map wrapped by this object.
  map<string, string> *obj = UnwrapMap(info.Holder());

  // Convert the key and value to std::strings.
  string key = ObjectToString(info.isolate_, Local<String>::Cast(name));
  string value = ObjectToString(info.isolate_, value_obj);

  // Update the map.
  (*obj)[key] = value;

  // Return the value; any non-empty handle will work.
  info.GetReturnValue().Set(value_obj);
}

Local<ObjectTemplate>
JSWorker::MakeMapTemplate(Isolate *isolate) {
  EscapableHandleScope handle_scope(isolate);

  Local<ObjectTemplate> result = ObjectTemplate::New(isolate);
  result->SetInternalFieldCount(1);
  result->SetHandler(NamedPropertyHandlerConfiguration(MapGet, MapSet));

  // Again, return the result through the current handle scope.
  return handle_scope.Escape(result);
}

Local<Object> JSWorker::WrapRequest(HttpRequest *request) {
  // Local scope for temporary handles.
  EscapableHandleScope handle_scope(isolate_);

  // Fetch the template for creating JavaScript http request wrappers.
  // It only has to be created once, which we do on demand.
  if (request_template_.IsEmpty()) {
    Local<ObjectTemplate> raw_template = MakeRequestTemplate(isolate_);
    request_template_.Reset(isolate_, raw_template);
  }
  Local<ObjectTemplate> templ =
      Local<ObjectTemplate>::New(isolate_, request_template_);

  // Create an empty http request wrapper.
  Local<Object> result =
      templ->NewInstance(isolate_->GetCurrentContext()).ToLocalChecked();

  // Wrap the raw C++ pointer in an External so it can be referenced
  // from within JavaScript.
  Local<External> request_ptr = External::New(isolate_, request);

  // Store the request pointer in the JavaScript wrapper.
  result->SetInternalField(0, request_ptr);

  // Return the result through the current handle scope.  Since each
  // of these handles will go away when the handle scope is deleted
  // we need to call Close to let one, the result, escape into the
  // outer handle scope.
  return handle_scope.Escape(result);
}

HttpRequest *JSWorker::UnwrapRequest(Local<Object> obj) {
  Local<External> field = Local<External>::Cast(obj->GetInternalField(0));
  void *ptr = field->Value();
  return static_cast<HttpRequest *>(ptr);
}

void JSWorker::GetPath(Local<String> name,
                                     const PropertyCallbackInfo<Value> &info) {
  // Extract the C++ request object from the JavaScript wrapper.
  HttpRequest *request = UnwrapRequest(info.Holder());

  // Fetch the path.
  const string &path = request->Path();

  // Wrap the result in a JavaScript string and return it.
  info.GetReturnValue().Set(String::NewFromUtf8(info.isolate_, path.c_str(),
                                                NewStringType::kNormal,
                                                static_cast<int>(path.length()))
                                .ToLocalChecked());
}

void JSWorker::GetReferrer(
    Local<String> name, const PropertyCallbackInfo<Value> &info) {
  HttpRequest *request = UnwrapRequest(info.Holder());
  const string &path = request->Referrer();
  info.GetReturnValue().Set(String::NewFromUtf8(info.isolate_, path.c_str(),
                                                NewStringType::kNormal,
                                                static_cast<int>(path.length()))
                                .ToLocalChecked());
}

void JSWorker::GetHost(Local<String> name,
                                     const PropertyCallbackInfo<Value> &info) {
  HttpRequest *request = UnwrapRequest(info.Holder());
  const string &path = request->Host();
  info.GetReturnValue().Set(String::NewFromUtf8(info.isolate_, path.c_str(),
                                                NewStringType::kNormal,
                                                static_cast<int>(path.length()))
                                .ToLocalChecked());
}

void JSWorker::GetUserAgent(
    Local<String> name, const PropertyCallbackInfo<Value> &info) {
  HttpRequest *request = UnwrapRequest(info.Holder());
  const string &path = request->UserAgent();
  info.GetReturnValue().Set(String::NewFromUtf8(info.isolate_, path.c_str(),
                                                NewStringType::kNormal,
                                                static_cast<int>(path.length()))
                                .ToLocalChecked());
}

Local<ObjectTemplate>
JSWorker::MakeRequestTemplate(Isolate *isolate) {
  EscapableHandleScope handle_scope(isolate);

  Local<ObjectTemplate> result = ObjectTemplate::New(isolate);
  result->SetInternalFieldCount(1);

  // Add accessors for each of the fields of the request.
  result->SetAccessor(
      String::NewFromUtf8(isolate, "path", NewStringType::kInternalized)
          .ToLocalChecked(),
      GetPath);
  result->SetAccessor(
      String::NewFromUtf8(isolate, "referrer", NewStringType::kInternalized)
          .ToLocalChecked(),
      GetReferrer);
  result->SetAccessor(
      String::NewFromUtf8(isolate, "host", NewStringType::kInternalized)
          .ToLocalChecked(),
      GetHost);
  result->SetAccessor(
      String::NewFromUtf8(isolate, "userAgent", NewStringType::kInternalized)
          .ToLocalChecked(),
      GetUserAgent);

  // Again, return the result through the current handle scope.
  return handle_scope.Escape(result);
}

// --- Test ---

void HttpRequestProcessor::Log(const char *event) {
  printf("Logged: %s\n", event);
}

class StringHttpRequest : public HttpRequest {
public:
  StringHttpRequest(const string &path, const string &referrer,
                    const string &host, const string &user_agent);
  virtual const string &Path() { return path_; }
  virtual const string &Referrer() { return referrer_; }
  virtual const string &Host() { return host_; }
  virtual const string &UserAgent() { return user_agent_; }

private:
  string path_;
  string referrer_;
  string host_;
  string user_agent_;
};

StringHttpRequest::StringHttpRequest(const string &path, const string &referrer,
                                     const string &host,
                                     const string &user_agent)
    : path_(path), referrer_(referrer), host_(host), user_agent_(user_agent) {}

void ParseOptions(int argc, char *argv[], map<string, string> *options,
                  string *file) {
  for (int i = 1; i < argc; i++) {
    string arg = argv[i];
    size_t index = arg.find('=', 0);
    if (index == string::npos) {
      *file = arg;
    } else {
      string key = arg.substr(0, index);
      string value = arg.substr(index + 1);
      (*options)[key] = value;
    }
  }
}


const int kSampleSize = 6;
StringHttpRequest kSampleRequests[kSampleSize] = {
    StringHttpRequest("/process.cc", "localhost", "google.com", "firefox"),
    StringHttpRequest("/", "localhost", "google.net", "firefox"),
    StringHttpRequest("/", "localhost", "google.org", "safari"),
    StringHttpRequest("/", "localhost", "yahoo.com", "ie"),
    StringHttpRequest("/", "localhost", "yahoo.com", "safari"),
    StringHttpRequest("/", "localhost", "yahoo.com", "firefox")};

bool ProcessEntries(v8::Platform *platform, HttpRequestProcessor *processor,
                    int count, StringHttpRequest *reqs) {
  for (int i = 0; i < count; i++) {
    bool result = processor->Process(&reqs[i]);
    while (v8::platform::PumpMessageLoop(platform, Isolate::GetCurrent()))
      continue;
    if (!result)
      return false;
  }
  return true;
}

void PrintMap(map<string, string> *m) {
  for (map<string, string>::iterator i = m->begin(); i != m->end(); i++) {
    pair<string, string> entry = *i;
    printf("%s: %s\n", entry.first.c_str(), entry.second.c_str());
  }
}

int main(int argc, char* argv[]) {
  v8::V8::InitializeICUDefaultLocation(argv[0]);
  v8::V8::InitializeExternalStartupData(argv[0]);
  v8::Platform* platform = v8::platform::CreateDefaultPlatform();
  v8::V8::InitializePlatform(platform);
  v8::V8::Initialize();
  map<string, string> options;
  string file;
  ParseOptions(argc, argv, &options, &file);
  if (file.empty()) {
    fprintf(stderr, "No script was specified.\n");
    return 1;
  }
  Isolate::CreateParams create_params;
  create_params.array_buffer_allocator =
      v8::ArrayBuffer::Allocator::NewDefaultAllocator();
  Isolate* isolate = Isolate::New(create_params);
  Isolate::Scope isolate_scope(isolate);
  HandleScope scope(isolate);
  Local<String> source;
  if (!ReadFile(isolate, file).ToLocal(&source)) {
    fprintf(stderr, "Error reading '%s'.\n", file.c_str());
    return 1;
  }
  JSWorker processor(isolate, source);
  map<string, string> output;
  if (!processor.Initialize(&options, &output)) {
    fprintf(stderr, "Error initializing processor.\n");
    return 1;
  }
  if (!ProcessEntries(platform, &processor, kSampleSize, kSampleRequests))
    return 1;
  PrintMap(&output);
}
*/

// Reads a file into a v8 string.
MaybeLocal<String> ReadFile(Isolate *isolate, const string &name) {
  FILE *file = fopen(name.c_str(), "rb");
  if (file == NULL)
    return MaybeLocal<String>();

  fseek(file, 0, SEEK_END);
  size_t size = ftell(file);
  rewind(file);

  char *chars = new char[size + 1];
  chars[size] = '\0';
  for (size_t i = 0; i < size;) {
    i += fread(&chars[i], 1, size - i, file);
    if (ferror(file)) {
      fclose(file);
      return MaybeLocal<String>();
    }
  }
  fclose(file);
  MaybeLocal<String> result = String::NewFromUtf8(
      isolate, chars, NewStringType::kNormal, static_cast<int>(size));
  delete[] chars;
  return result;
}

extern "C" {

void initialize() {
  v8::V8::InitializeICUDefaultLocation("nuclio");
  v8::V8::InitializeExternalStartupData("nuclio");
  v8::Platform *platform = v8::platform::CreateDefaultPlatform();
  v8::V8::InitializePlatform(platform);
  v8::V8::Initialize();
}

new_result_t new_worker(char *code, char *handler_name) {
  new_result_t result;
  result.worker = NULL;
  result.error_message = NULL;

  Isolate::CreateParams create_params;
  create_params.array_buffer_allocator =
      v8::ArrayBuffer::Allocator::NewDefaultAllocator();
  Isolate* isolate = Isolate::New(create_params);
  Isolate::Scope isolate_scope(isolate);
  HandleScope scope(isolate);

  Local<String> source = String::NewFromUtf8(
      isolate, code, NewStringType::kNormal).ToLocalChecked();
  Local<String> handler = String::NewFromUtf8(
      isolate, handler_name, NewStringType::kNormal).ToLocalChecked();

  JSWorker* worker = new JSWorker(isolate, source, handler);
  /*
  map<string, string> output;
  if (!processor.Initialize(&options, &output)) {
  */
  char *error = worker->Initialize();
  if (error != NULL) {
    result.error_message = error;
    return result;
  }

  result.worker = worker;
  return result;
}

} // extern "C"
