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

func (ds *AbstractSync) GetMethod() string {
	return ""
}

func (ds *AbstractSync) GetPath() string {
	return ""
}

func (ds *AbstractSync) GetHostAddress() string {
	return ""
}

func (ds *AbstractSync) GetRemoteAddress() string {
	return ""
}

func (ds *AbstractSync) GetWorkflowStep() string {
	return ""
}

func (ds *AbstractSync) GetQuery() map[string]interface{} {
	return map[string]interface{}{}
}
