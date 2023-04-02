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
	Override       = "x-projects-role"

	// Others
	Logs           = "X-nuclio-logs"
	FilterContains = "x-nuclio-filter-contains"
	StreamNoAck    = "x-nuclio-stream-no-ack"
	Arguments      = "x-nuclio-arguments"
)

func IsNuclioHeader(headerName string) bool {
	return strings.HasPrefix(headerName, HeaderPrefix)
}
