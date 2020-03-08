package common

type ReusedMessage string

const (
	UnexpectedTerminationChildProcess ReusedMessage = "Unexpected termination of child process"
	FailedReadFromConnection ReusedMessage = "Failed to read from connection"
)
