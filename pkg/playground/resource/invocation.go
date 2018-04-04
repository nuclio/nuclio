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
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/playground"
	"github.com/nuclio/nuclio/pkg/restful"
)

type invocationResource struct {
	*resource
}

// called after initialization
func (tr *invocationResource) OnAfterInitialize() error {

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

	return nil
}

func (tr *invocationResource) handleRequest(responseWriter http.ResponseWriter, request *http.Request) {
	path := request.Header.Get("x-nuclio-path")
	functionName := request.Header.Get("x-nuclio-function-name")
	functionNamespace := request.Header.Get("x-nuclio-function-namespace")

	// set default namespace
	if functionNamespace == "" {
		functionNamespace = "default"
	}

	// if user prefixed path with "/", remove it
	path = strings.TrimLeft(path, "/")

	requestBody, err := ioutil.ReadAll(request.Body)
	if err != nil {
		responseWriter.WriteHeader(http.StatusInternalServerError)
		responseWriter.Write([]byte(`{"error": "Failed to read request body"}`)) // nolint: errcheck
		return
	}

	// resolve the function host
	invocationResult, err := tr.getPlatform().CreateFunctionInvocation(&platform.CreateFunctionInvocationOptions{
		Name:      functionName,
		Namespace: functionNamespace,
		Path:      path,
		Method:    request.Method,
		Headers:   request.Header,
		Body:      requestBody,
		Via:       platform.InvokeViaDomainName,
	})

	if err != nil {
		tr.Logger.WarnWith("Failed to invoke function", "err", err)

		responseWriter.WriteHeader(http.StatusInternalServerError)
		responseWriter.Write([]byte(`{"error": "Failed to invoke function"}`)) // nolint: errcheck
		return
	}

	// set headers
	for headerName, headerValue := range invocationResult.Headers {
		responseWriter.Header().Set(headerName, headerValue[0])
	}

	responseWriter.WriteHeader(invocationResult.StatusCode)
	responseWriter.Write(invocationResult.Body) // nolint: errcheck
}

// register the resource
var invocationResourceInstance = &invocationResource{
	resource: newResource("invocations", []restful.ResourceMethod{}),
}

func init() {
	invocationResourceInstance.Resource = invocationResourceInstance
	invocationResourceInstance.Register(playground.PlaygroundResourceRegistrySingleton)
}
