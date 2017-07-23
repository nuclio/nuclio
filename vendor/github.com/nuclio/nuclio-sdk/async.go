package nuclio

type Async interface {
	Event
	GetType() string
	GetApplicationID() string
	GetRetryCount() int
	GetReplyTo() string
	GetWorkflow() string
	GetWorkflowStep() string
	GetWorkflowIndex() string
}
