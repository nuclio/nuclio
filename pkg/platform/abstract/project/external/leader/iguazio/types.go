package iguazio

import (
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
)

const (
	ProjectType = "project"
)

type Project struct {
	Data ProjectData `json:"data,omitempty"`
}

func CreateProjectFromProjectConfig(projectConfig *platform.ProjectConfig) Project {
	return Project{
		Data: ProjectData{
			Type: ProjectType,
			Attributes: ProjectAttributes{
				Name:        projectConfig.Meta.Name,
				Labels:      labelMapToList(projectConfig.Meta.Labels),
				Annotations: labelMapToList(projectConfig.Meta.Annotations),
				Description: projectConfig.Spec.Description,
			},
		},
	}
}

func (pl *Project) GetConfig() *platform.ProjectConfig {
	return &platform.ProjectConfig{
		Meta: platform.ProjectMeta{
			Name:        pl.Data.Attributes.Name,
			Namespace:   pl.Data.Attributes.Namespace,
			Annotations: pl.labelListToMap(pl.Data.Attributes.Annotations),
			Labels:      pl.labelListToMap(pl.Data.Attributes.Labels),
		},
		Spec: platform.ProjectSpec{
			Description: pl.Data.Attributes.Description,
		},
		Status: platform.ProjectStatus{
			AdminStatus:       pl.Data.Attributes.AdminStatus,
			OperationalStatus: pl.Data.Attributes.OperationalStatus,
			UpdatedAt:         pl.parseTimeFromTimestamp(pl.Data.Attributes.UpdatedAt),
		},
	}
}

func labelMapToList(labelMap map[string]string) []Label {
	var labelList []Label

	for labelName, labelValue := range labelMap {
		labelList = append(labelList, Label{Name: labelName, Value: labelValue})
	}

	return labelList
}

func (pl *Project) parseTimeFromTimestamp(timestamp string) time.Time {
	loc, _ := time.LoadLocation("GMT")
	layout := "2006-01-02T15:04:05.000000+00:00"
	t, _ := time.ParseInLocation(layout, timestamp, loc)
	return t
}

func (pl *Project) labelListToMap(labelList []Label) map[string]string {
	labelsMap := map[string]string{}

	for _, label := range labelList {
		labelsMap[label.Name] = label.Value
	}

	return labelsMap
}

type ProjectData struct {
	Type       string            `json:"type,omitempty"`
	Attributes ProjectAttributes `json:"attributes,omitempty"`
}

type ProjectAttributes struct {
	Name              string        `json:"name,omitempty"`
	Namespace         string        `json:"namespace,omitempty"`
	Labels            []Label       `json:"labels,omitempty"`
	Annotations       []Label       `json:"annotations,omitempty"`
	Description       string        `json:"description,omitempty"`
	AdminStatus       string        `json:"admin_status,omitempty"`
	OperationalStatus string        `json:"operational_status,omitempty"`
	UpdatedAt         string        `json:"updated_at,omitempty"`
	NuclioProject     NuclioProject `json:"nuclio_project,omitempty"`
}

type Label struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type NuclioProject struct {
	// currently no nuclio specific fields are needed
}

type ProjectList struct {
	Data []ProjectData `json:"data,omitempty"`
}

// ProjectList -> []Project
func (pl *ProjectList) ToSingleProjectList() []platform.Project {
	var projects []platform.Project

	for _, projectData := range pl.Data {
		projects = append(projects, &Project{Data: projectData})
	}

	return projects
}
