package iguazio

import (
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
)

const (
	ProjectType       = "project"
	ProjectTimeLayout = "2006-01-02T15:04:05.000000+00:00"
)

type Project struct {
	Data ProjectData `json:"data,omitempty"`
}

func NewProjectFromProjectConfig(projectConfig *platform.ProjectConfig) Project {
	return Project{
		Data: ProjectData{
			Type: ProjectType,
			Attributes: ProjectAttributes{
				Name:          projectConfig.Meta.Name,
				Labels:        labelMapToList(projectConfig.Meta.Labels),
				Annotations:   labelMapToList(projectConfig.Meta.Annotations),
				Description:   projectConfig.Spec.Description,
				OwnerUsername: projectConfig.Spec.Owner,
			},
		},
	}
}

func (pl *Project) GetConfig() *platform.ProjectConfig {
	updatedAt := pl.parseTimeFromTimestamp(pl.Data.Attributes.UpdatedAt)
	return &platform.ProjectConfig{
		Meta: platform.ProjectMeta{
			Name:        pl.Data.Attributes.Name,
			Namespace:   pl.Data.Attributes.Namespace,
			Annotations: labelListToMap(pl.Data.Attributes.Annotations),
			Labels:      labelListToMap(pl.Data.Attributes.Labels),
		},
		Spec: platform.ProjectSpec{
			Description: pl.Data.Attributes.Description,
			Owner:       pl.Data.Attributes.OwnerUsername,
		},
		Status: platform.ProjectStatus{
			AdminStatus:       pl.Data.Attributes.AdminStatus,
			OperationalStatus: pl.Data.Attributes.OperationalStatus,
			UpdatedAt:         &updatedAt,
		},
	}
}

func (pl *Project) parseTimeFromTimestamp(timestamp string) time.Time {
	t, _ := time.Parse(ProjectTimeLayout, timestamp)
	return t
}

type ProjectData struct {
	Type          string                `json:"type,omitempty"`
	Attributes    ProjectAttributes     `json:"attributes,omitempty"`
	Relationships *ProjectRelationships `json:"relationships,omitempty"`
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
	OwnerUsername     string        `json:"owner_username,omitempty"`
}

type ProjectRelationships struct {
	LastJob struct {
		Data struct {
			ID string `json:"id,omitempty"`
		} `json:"data,omitempty"`
	} `json:"last_job,omitempty"`
}

type Label struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type NuclioProject struct {
	// currently no nuclio specific fields are needed
}

type JobState string

const (
	JobStateCompleted JobState = "completed"
	JobStateCanceled  JobState = "canceled"
	JobStateFailed    JobState = "failed"
)

func JobStateInSlice(jobState JobState, slice []JobState) bool {
	for _, otherJobState := range slice {
		if otherJobState == jobState {
			return true
		}
	}
	return false

}

type JobDetail struct {
	Data JobData `json:"data,omitempty"`
}

type JobData struct {
	Type       string        `json:"type,omitempty"`
	Attributes JobAttributes `json:"attributes,omitempty"`
}

type JobAttributes struct {
	Kind  string   `json:"kind,omitempty"`
	State JobState `json:"state,omitempty"`
}

type GetProjectResponse interface {
	ToSingleProjectList() []platform.Project
}

type ProjectList struct {
	Data []ProjectData `json:"data,omitempty"`
}

// ToSingleProjectList returns list of Project
func (pl *ProjectList) ToSingleProjectList() []platform.Project {
	var projects []platform.Project

	for _, projectData := range pl.Data {
		projects = append(projects, &Project{Data: projectData})
	}

	return projects
}

type ProjectDetail struct {
	Data ProjectData `json:"data,omitempty"`
}

// ToSingleProjectList returns list of Project
func (pl *ProjectDetail) ToSingleProjectList() []platform.Project {
	return []platform.Project{
		&Project{Data: pl.Data},
	}
}
