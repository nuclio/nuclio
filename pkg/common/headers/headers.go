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

package headers

import "strings"

// Nuclio Headers
const (
	HeaderPrefix = "X-Nuclio"

	// Function headers
	FunctionName                        = "X-Nuclio-Function-Name"
	FunctionNamespace                   = "X-Nuclio-Function-Namespace"
	WaitFunctionAction                  = "X-Nuclio-Wait-Function-Action"
	DeleteFunctionIgnoreStateValidation = "X-Nuclio-Delete-Function-Ignore-State-Validation"
	DeleteFunctionWithGateways          = "X-Nuclio-Delete-Function-With-API-Gateways"
	CreationStateUpdatedTimeout         = "X-Nuclio-Creation-State-Updated-Timeout"
	FunctionEnrichApiGateways           = "X-Nuclio-Function-Enrich-Apigateways"
	ImportedFunctionOnly                = "X-Nuclio-Imported-Function-Only"
	SkipSpecCleanup                     = "X-Nuclio-Skip-Spec-Cleanup"
	VerifyExternalRegistry              = "X-Nuclio-Verify-External-Registry"
	AutofixFunctionConfiguration        = "X-Nuclio-Autofix-Function-Configuration"

	// Project headers
	ProjectName           = "X-Nuclio-Project-Name"
	ProjectNamespace      = "X-Nuclio-Project-Namespace"
	DeleteProjectStrategy = "X-Nuclio-Delete-Project-Strategy"

	// Invocation headers
	TargetName          = "X-Nuclio-Target"
	InvokeURL           = "X-Nuclio-Invoke-Url"
	InvokeTimeout       = "X-Nuclio-Invoke-Timeout"
	InvokeVia           = "X-Nuclio-Invoke-Via"
	SkipTLSVerification = "X-Nuclio-Skip-Tls-Verification"
	Path                = "X-Nuclio-Path"
	LogLevel            = "X-Nuclio-Log-Level"

	// ApiGateway headers
	ApiGatewayName                      = "X-Nuclio-Api-Gateway-Name"
	ApiGatewayNamespace                 = "X-Nuclio-Api-Gateway-Namespace"
	ApiGatewayValidateFunctionExistence = "X-Nuclio-Agw-Validate-Functions-Existence"

	// Function event headers
	FunctionEventName      = "X-Nuclio-Function-Event-Name"
	FunctionEventNamespace = "X-Nuclio-Function-Event-Namespace"

	// Auth headers
	RemoteUser     = "X-Remote-User"
	V3IOSessionKey = "X-V3io-Session-Key"
	UserID         = "X-User-Id"
	UserGroupIds   = "X-User-Group-Ids"

	// Others
	Logs           = "X-Nuclio-Logs"
	FilterContains = "X-Nuclio-Filter-Contains"
	StreamNoAck    = "X-Nuclio-Stream-No-Ack"
	Arguments      = "X-Nuclio-Arguments"
)

func IsNuclioHeader(headerName string) bool {
	return strings.HasPrefix(headerName, HeaderPrefix)
}
