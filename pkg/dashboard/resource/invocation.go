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

	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/platform"
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
	invokeURL := request.Header.Get("x-nuclio-invoke-url")
	invokeVia := tr.getInvokeVia(request.Header.Get("x-nuclio-invoke-via"))

	// get namespace from request or use the provided default
	functionNamespace := tr.getNamespaceOrDefault(request.Header.Get("x-nuclio-function-namespace"))

	// if user prefixed path with "/", remove it
	path = strings.TrimLeft(path, "/")

	if functionName == "" || functionNamespace == "" {
		tr.writeErrorHeader(responseWriter, http.StatusBadRequest)
		responseWriter.Write([]byte(`{"error": "Function name must be provided"}`)) // nolint: errcheck
		return
	}

	requestBody, err := ioutil.ReadAll(request.Body)
	if err != nil {
		tr.writeErrorHeader(responseWriter, http.StatusInternalServerError)
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
		Via:       invokeVia,
		URL:       invokeURL,
	})

	if err != nil {
		tr.Logger.WarnWith("Failed to invoke function", "err", err)

		tr.writeErrorHeader(responseWriter, http.StatusInternalServerError)
		responseWriter.Write([]byte(`{"error": "Failed to invoke function"}`)) // nolint: errcheck
		return
	}

	// set headers
	for headerName, headerValue := range invocationResult.Headers {

		// don't send nuclio headers to the actual function
		if !strings.HasPrefix(headerName, "x-nuclio") {
			responseWriter.Header().Set(headerName, headerValue[0])
		}
	}

	responseWriter.WriteHeader(invocationResult.StatusCode)
	responseWriter.Write(invocationResult.Body) // nolint: errcheck
}

func (tr *invocationResource) getInvokeVia(invokeViaName string) platform.InvokeViaType {
	switch invokeViaName {
	// erd: For now, if the UI asked for external IP, force using "via any". "Any" should try external IP
	// and then domain name, which is better
	// case "external-ip":
	// 	 return platform.InvokeViaExternalIP
	case "loadbalancer":
		return platform.InvokeViaLoadBalancer
	case "domain-name":
		return platform.InvokeViaDomainName
	default:
		return platform.InvokeViaAny
	}
}

func (tr *invocationResource) writeErrorHeader(responseWriter http.ResponseWriter, statusCode int) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
}

// register the resource
var invocationResourceInstance = &invocationResource{
	resource: newResource("api/function_invocations", []restful.ResourceMethod{}),
}

func init() {
	invocationResourceInstance.Resource = invocationResourceInstance
	invocationResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
