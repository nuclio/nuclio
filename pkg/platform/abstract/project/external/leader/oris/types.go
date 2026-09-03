/*
Copyright 2026 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package oris

import (
	"github.com/nuclio/nuclio/pkg/platform"
	common "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"

	"github.com/nuclio/errors"
)

// OrisProject represents a project as returned by / sent to the Oris leader.
type OrisProject struct {
	Metadata OrisProjectMetadata `json:"metadata"`
	Spec     OrisProjectSpec     `json:"spec"`
	Status   OrisProjectStatus   `json:"status"`
}

// OrisProjectRequest is the flat wire shape Oris expects for CreateProject/UpdateProject
// requests: its gRPC proxy unmarshals the request body directly into the (unnested)
// CreateProjectOptions/UpdateProjectOptions proto message.
type OrisProjectRequest struct {
	Name        string            `json:"name"`
	Owner       string            `json:"owner,omitempty"`
	Description string            `json:"description,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// GetLastJobID satisfies the leader.CreateProjectResponse interface.
// TODO: Oris does not yet have async job handling; revisit if it gains one.
func (p *OrisProject) GetLastJobID() string {
	return ""
}

// GetConfig satisfies the platform.Project interface.
func (p *OrisProject) GetConfig() *platform.ProjectConfig {
	updatedAt := common.ParseTimeFromTimestamp(p.Metadata.UpdatedAt)
	return &platform.ProjectConfig{
		Meta: platform.ProjectMeta{
			Name:        p.Metadata.Name,
			Namespace:   p.Metadata.Namespace,
			Labels:      p.Metadata.Labels,
			Annotations: p.Metadata.Annotations,
		},
		Spec: platform.ProjectSpec{
			Description: p.Spec.Description,
			Owner:       p.Spec.Owner,
		},
		Status: platform.ProjectStatus{
			UpdatedAt: &updatedAt,
		},
	}
}

// IsProjectOnline satisfies the platform.Project interface.
func (p *OrisProject) IsProjectOnline() bool {
	// TODO: Resolve the real online status according to the Oris leader.
	return true
}

// NewProjectRequestFromProjectConfig converts a platform.ProjectConfig into the flat Oris request wire format.
func NewProjectRequestFromProjectConfig(projectConfig *platform.ProjectConfig) (OrisProjectRequest, error) {
	if projectConfig == nil {
		return OrisProjectRequest{}, errors.New("ProjectConfig is nil")
	}

	return OrisProjectRequest{
		Name:        projectConfig.Meta.Name,
		Owner:       projectConfig.Spec.Owner,
		Description: projectConfig.Spec.Description,
		Labels:      projectConfig.Meta.Labels,
		Annotations: projectConfig.Meta.Annotations,
	}, nil
}

type OrisProjectMetadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	UpdatedAt   string            `json:"updatedAt,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type OrisProjectSpec struct {
	Owner       string `json:"owner,omitempty"`
	Description string `json:"description,omitempty"`
}

type OrisProjectStatus struct {
	Ctx          string `json:"ctx,omitempty"`
	StatusCode   int32  `json:"statusCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	StackTrace   string `json:"stackTrace,omitempty"`
	RedirectUri  string `json:"redirectUri,omitempty"`
}

type OrisListStatus struct {
	*OrisProjectStatus
	Total   int32 `json:"total,omitempty"`
	HasMore bool  `json:"hasMore,omitempty"`
}

// OrisProjectList is a placeholder shape for a list-projects response from the Oris leader.
type OrisProjectList struct {
	Items  []OrisProject  `json:"items,omitempty"`
	Status OrisListStatus `json:"status,omitempty"`
}

// ToProjectList converts an OrisProjectList into the generic platform.Project slice,
// enriching each entry with namespace since the leader response may omit it.
func (opl *OrisProjectList) ToProjectList(namespace string) []platform.Project {
	var projects []platform.Project
	for _, orisProject := range opl.Items {
		project := &OrisProject{
			Metadata: orisProject.Metadata,
			Spec:     orisProject.Spec,
			Status:   orisProject.Status,
		}
		project.Metadata.Namespace = namespace
		projects = append(projects, project)
	}
	return projects
}
