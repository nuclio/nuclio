package event

type Sync interface {
	Event
	GetMethod() string
	GetPath() string
	GetHostAddress() string
	GetRemoteAddress() string
	GetWorkflowStep() string
	GetQuery() map[string]interface{}
}

type AbstractSync struct {
	AbstractEvent
}

func (as *AbstractSync) GetMethod() string {
	return ""
}

func (as *AbstractSync) GetPath() string {
	return ""
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
