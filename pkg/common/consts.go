package common

type ReusedMessage string

const (
	UnexpectedTerminationChildProcess ReusedMessage = "Unexpected termination of child process"
	WorkDirectoryDoesNotExist         ReusedMessage = "Work directory does not exist"
	WorkDirectoryExpectedBeString     ReusedMessage = "Work directory is expected to be string"
)
