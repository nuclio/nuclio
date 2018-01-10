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

package resource

import (
	"net/http"

	"github.com/nuclio/nuclio/pkg/playground"
	"github.com/nuclio/nuclio/pkg/restful"
)

type invocationResource struct {
	*resource
}

// called after initialization
func (tr *invocationResource) OnAfterInitialize() {

	// all methods
	for _, registrar := range []func(string, http.HandlerFunc){
		tr.GetRouter().Get,
		tr.GetRouter().Post,
		tr.GetRouter().Put,
		tr.GetRouter().Delete,
		tr.GetRouter().Patch,
		tr.GetRouter().Options,
	} {
		registrar("/*", tr.handleRequest)
	}
}

func (tr *invocationResource) handleRequest(responseWriter http.ResponseWriter, request *http.Request) {
	//
	//path := request.Header.Get("x-nuclio-path")
	//invokerIPAddress := request.Header.Get("x-nuclio-invoker-ip-address")
	//functionNodePort := request.Header.Get("x-nuclio-function-nodeport")
	//functionName := request.Header.Get("x-nuclio-function-name")
	//functionNamespace := request.Header.Get("x-nuclio-function-namespace")
	//
	//// resolve the function host
	//invocationResult, err := tr.getPlatform().InvokeFunction(&platform.InvokeOptions{
	//	Name: functionName,
	//	Namespace: functionNamespace,
	//	ContentType: request.Header.Get("content-type"),
	//	Path: path,
	//	Method: request.Method,
	//	Body: "this is the body",
	//	Via: platform.InvokeViaDomainName,
	//})
	//
	//// get host and URL from request
	//host, path, err := tr.getTunneledHostAndPath(request.URL.Path)
	//if err != nil {
	//	responseWriter.Write([]byte(`{"error": Invalid path - expected /invocation/<host>/<path>""}`))
	//	responseWriter.WriteHeader(http.StatusBadRequest)
	//	return
	//}
	//
	//tr.Logger.DebugWith("Tunneling request",
	//	"url", request.URL.Path,
	//	"host", host,
	//	"path", path)
	//
	//// create a url from the path
	//fullURL := fmt.Sprintf("http://%s%s", host, path)
	//
	//// create a request
	//invocationedRequest, err := http.NewRequest(request.Method, fullURL, request.Body)
	//if err != nil {
	//	tr.Logger.WarnWith("Failed to create invocationed request", "err", err)
	//	responseWriter.WriteHeader(http.StatusInternalServerError)
	//	return
	//}
	//
	//// set headers
	//invocationedRequest.Header = request.Header
	//
	//// do the invocationing
	//invocationedHTTPResponse, err := http.DefaultClient.Do(invocationedRequest)
	//if err != nil {
	//	tr.Logger.WarnWith("Failed to invocation request", "err", err)
	//	responseWriter.WriteHeader(http.StatusInternalServerError)
	//	return
	//}
	//
	//responseBody, err := ioutil.ReadAll(invocationedHTTPResponse.Body)
	//if err != nil {
	//	tr.Logger.WarnWith("Failed to read invocationed response", "err", err)
	//	responseWriter.WriteHeader(http.StatusInternalServerError)
	//	return
	//}
	//
	//// set headers
	//for headerName, headerValue := range invocationedHTTPResponse.Header {
	//	responseWriter.Header()[headerName] = headerValue
	//}
	//
	//responseWriter.WriteHeader(invocationedHTTPResponse.StatusCode)
	//responseWriter.Write(responseBody)
}

// register the resource
var invocationResourceInstance = &invocationResource{
	resource: newResource("invocation", []restful.ResourceMethod{}),
}

func init() {
	invocationResourceInstance.Resource = invocationResourceInstance
	invocationResourceInstance.Register(playground.PlaygroundResourceRegistrySingleton)
}
