package iguazio

const (
	ProjectType = "project"
)

type Project struct {
	Data ProjectData `json:"data,omitempty"`
}

type ProjectData struct {
	Type string      `json:"type,omitempty"`
	Attributes ProjectAttributes `json:"attributes,omitempty"`
}

type ProjectAttributes struct {
	Name         string            `json:"name,omitempty"`
	Namespace    string            `json:"namespace,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	Description  string            `json:"description,omitempty"`
	NuclioFields NuclioProject     `json:"nuclio_project,omitempty"`
}

type NuclioProject struct {
	// currently no nuclio specific fields are needed
}
