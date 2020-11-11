package common

type ReusedMessage string

const (
	UnexpectedTerminationChildProcess ReusedMessage = "Unexpected termination of child process"
	WorkDirectoryDoesNotExist         ReusedMessage = "Work directory does not exist"
	WorkDirectoryExpectedBeString     ReusedMessage = "Work directory is expected to be string"
	FailedReadFromConnection          ReusedMessage = "Failed to read from connection"
)

type FunctionStateMessage string

const (
	FunctionStateMessageUnhealthy = "Function is not healthy"

	// TODO: deprecated. (used by local platform)
	DeprecatedFunctionStateMessage = "Container is not healthy (detected by nuclio platform)"
)
