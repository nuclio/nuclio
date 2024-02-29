/*
Copyright 2023 The Nuclio Authors.

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
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/opa"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
)

type invocationResource struct {
	*resource
}

func (tr *invocationResource) ExtendMiddlewares() error {
	tr.resource.addAuthMiddleware(nil)
	return nil
}

// OnAfterInitialize is called after initialization
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
	ctx := request.Context()
	path := request.Header.Get(headers.Path)
	functionName := request.Header.Get(headers.FunctionName)
	invokeURL := request.Header.Get(headers.InvokeURL)

	// get namespace from request or use the provided default
	functionNamespace := tr.getNamespaceOrDefault(request.Header.Get(headers.FunctionNamespace))

	// if user prefixed path with "/", remove it
	path = strings.TrimLeft(path, "/")

	if functionName == "" || functionNamespace == "" {
		tr.writeErrorHeader(responseWriter, http.StatusBadRequest)
		tr.writeErrorMessage(responseWriter, "Function name must be provided")
		return
	}

	requestBody, err := io.ReadAll(request.Body)
	if err != nil {
		tr.writeErrorHeader(responseWriter, http.StatusInternalServerError)
		tr.writeErrorMessage(responseWriter, "Failed to read request body")
		return
	}

	invokeTimeout, err := tr.resolveInvokeTimeout(request.Header.Get(headers.InvokeTimeout))
	if err != nil {
		tr.writeErrorHeader(responseWriter, http.StatusBadRequest)
		tr.writeErrorMessage(responseWriter, errors.RootCause(err).Error())
	}

	skipTLSVerification := strings.ToLower(request.Header.Get(headers.SkipTLSVerification)) == "true"

	// resolve the function host
	invocationResult, err := tr.getPlatform().CreateFunctionInvocation(ctx, &platform.CreateFunctionInvocationOptions{
		Name:                functionName,
		Namespace:           functionNamespace,
		Path:                path,
		Method:              request.Method,
		Headers:             request.Header,
		Body:                requestBody,
		URL:                 invokeURL,
		Timeout:             invokeTimeout,
		SkipTLSVerification: skipTLSVerification,

		// auth & permissions
		AuthSession: tr.getCtxSession(ctx),
		PermissionOptions: opa.PermissionOptions{
			MemberIds:           opa.GetUserAndGroupIdsFromAuthSession(tr.getCtxSession(ctx)),
			RaiseForbidden:      true,
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
	})

	if err != nil {
		tr.Logger.WarnWithCtx(ctx, "Failed to invoke function", "err", err)
		tr.writeErrorHeader(responseWriter, common.ResolveErrorStatusCodeOrDefault(err, http.StatusInternalServerError))
		tr.writeErrorMessage(responseWriter, fmt.Sprintf("Failed to invoke function: %v", errors.RootCause(err)))
		return
	}

	// set headers
	for headerName, headerValue := range invocationResult.Headers {

		// don't send nuclio headers to the actual function
		if !headers.IsNuclioHeader(headerName) {
			responseWriter.Header().Set(headerName, headerValue[0])
		}
	}

	sanitizedBody := common.SanitizeResponseData(invocationResult.Body, invocationResult.Headers)

	responseWriter.WriteHeader(invocationResult.StatusCode)
	responseWriter.Write(sanitizedBody) // nolint: errcheck
}

func (tr *invocationResource) writeErrorHeader(responseWriter http.ResponseWriter, statusCode int) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
}

func (tr *invocationResource) writeErrorMessage(responseWriter io.Writer, message string) {

	// replace " with ' so that the error message will be valid json
	formattedMessage := fmt.Sprintf(`{"error": "%s"}`, strings.ReplaceAll(message, `"`, `'`))
	responseWriter.Write([]byte(formattedMessage)) // nolint: errcheck
}

func (tr *invocationResource) resolveInvokeTimeout(invokeTimeout string) (time.Duration, error) {
	if invokeTimeout == "" {
		return tr.getPlatform().GetConfig().GetDefaultFunctionInvocationTimeout(), nil
	}
	parsedDuration, err := time.ParseDuration(invokeTimeout)
	if err != nil {
		return 0, nuclio.NewErrBadRequest("Invalid invoke timeout")
	}
	return parsedDuration, nil
}

// register the resource
var invocationResourceInstance = &invocationResource{
	resource: newResource("api/function_invocations", []restful.ResourceMethod{}),
}

func init() {
	invocationResourceInstance.Resource = invocationResourceInstance
	invocationResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
