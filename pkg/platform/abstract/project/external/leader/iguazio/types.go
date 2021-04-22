package iguazio

const (
	ProjectType = "project"
)

type Project struct {
	Type string      `json:"type,omitempty"`
	Data ProjectData `json:"data,omitempty"`
}

type ProjectData struct {
	Attributes ProjectAttributes `json:"attributes,omitempty"`
}

type ProjectAttributes struct {
	Name         string            `json:"name,omitempty"`
	Namespace    string            `json:"namespace,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	Description  string            `json:"description,omitempty"`
	NuclioFields NuclioFields      `json:"nuclio_fields,omitempty"`
}

type NuclioFields struct {
	// currently no nuclio specific fields are needed
}
