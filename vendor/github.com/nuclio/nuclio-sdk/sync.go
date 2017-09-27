package nuclio

// Sync is synchronic event interface
type Sync interface {
	Event
	GetHostAddress() string
	GetRemoteAddress() string
	GetWorkflowStep() string
	GetQuery() map[string]interface{}
}
