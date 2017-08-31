package nuclio

type Sync interface {
	Event
	GetHostAddress() string
	GetRemoteAddress() string
	GetWorkflowStep() string
	GetQuery() map[string]interface{}
}

type AbstractSync struct {
	AbstractEvent
}

func (as *AbstractSync) GetHostAddress() string {
	return ""
}

func (as *AbstractSync) GetRemoteAddress() string {
	return ""
}

func (as *AbstractSync) GetWorkflowStep() string {
	return ""
}

func (as *AbstractSync) GetQuery() map[string]interface{} {
	return map[string]interface{}{}
}
