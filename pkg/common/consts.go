package common

type ReusedMessage string

const (
	UnexpectedTerminationChildProcess ReusedMessage = "Unexpected termination of child process"
	WorkDirectoryDoesNotExist         ReusedMessage = "Work directory does not exist"
	WorkDirectoryExpectedBeString     ReusedMessage = "Work directory is expected to be string"
	FailedReadFromConnection          ReusedMessage = "Failed to read from connection"
	FailedReadControlMessage          ReusedMessage = "Failed to read control message"
)

type FunctionStateMessage string

const (
	FunctionStateMessageUnhealthy FunctionStateMessage = "Function is not healthy"
)

type NuclioResourceLabelKey string

const NuclioResourceLabelKeyProjectName = "nuclio.io/project-name"
const NuclioResourceLabelKeyFunctionName = "nuclio.io/function-name"
const NuclioResourceLabelKeyApiGatewayName = "nuclio.io/apigateway-name"

// KubernetesDomainLevelMaxLength DNS domain level limitation is 63 chars
// https://en.wikipedia.org/wiki/Subdomain#Overview
const KubernetesDomainLevelMaxLength = 63
