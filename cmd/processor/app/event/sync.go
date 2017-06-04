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

type DefaultSync struct {
	DefaultEvent
}

func (ds *DefaultSync) GetMethod() string {
	return ""
}

func (ds *DefaultSync) GetPath() string {
	return ""
}

func (ds *DefaultSync) GetHostAddress() string {
	return ""
}

func (ds *DefaultSync) GetRemoteAddress() string {
	return ""
}

func (ds *DefaultSync) GetWorkflowStep() string {
	return ""
}

func (ds *DefaultSync) GetQuery() map[string]interface{} {
	return map[string]interface{}{}
}
