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

package headers

import "strings"

// Nuclio Headers
const (
	HeaderPrefix = "x-nuclio"

	// Function headers
	FunctionName                        = "x-nuclio-function-name"
	FunctionNamespace                   = "x-nuclio-function-namespace"
	WaitFunctionAction                  = "x-nuclio-wait-function-action"
	DeleteFunctionIgnoreStateValidation = "x-nuclio-delete-function-ignore-state-validation"
	CreationStateUpdatedTimeout         = "X-nuclio-creation-state-updated-timeout"
	FunctionEnrichApiGateways           = "X-nuclio-function-enrich-apigateways"

	// Project headers
	ProjectName           = "x-nuclio-project-name"
	ProjectNamespace      = "X-nuclio-project-namespace"
	DeleteProjectStrategy = "X-nuclio-delete-project-strategy"

	// Invocation headers
	TargetName          = "x-nuclio-target"
	InvokeURL           = "x-nuclio-invoke-url"
	InvokeTimeout       = "X-nuclio-invoke-timeout"
	InvokeVia           = "X-nuclio-invoke-via"
	SkipTLSVerification = "x-nuclio-skip-tls-verification"
	Path                = "x-nuclio-path"
	LogLevel            = "X-nuclio-log-level"

	// ApiGateway headers
	ApiGatewayName                      = "X-nuclio-api-gateway-name"
	ApiGatewayNamespace                 = "x-nuclio-api-gateway-namespace"
	ApiGatewayValidateFunctionExistence = "x-nuclio-agw-validate-functions-existence"

	// Function event headers
	FunctionEventName      = "X-nuclio-function-event-name"
	FunctionEventNamespace = "X-nuclio-function-event-namespace"

	// Auth headers
	RemoteUser     = "x-remote-user"
	V3IOSessionKey = "x-v3io-session-key"
	UserID         = "x-user-id"
	UserGroupIds   = "x-user-group-ids"

	// Others
	Logs           = "X-nuclio-logs"
	FilterContains = "x-nuclio-filter-contains"
	StreamNoAck    = "x-nuclio-stream-no-ack"
	Arguments      = "x-nuclio-arguments"
)

func IsNuclioHeader(headerName string) bool {
	return strings.HasPrefix(headerName, HeaderPrefix)
}
