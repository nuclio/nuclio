/*
Copyright 2025 The Nuclio Authors.

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

package mlrun

import (
	"github.com/nuclio/nuclio/pkg/platform"
	common "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"

	"github.com/nuclio/errors"
)

type MLRunProject struct {
	Metadata ProjectMetadata `json:"metadata"`
	Spec     ProjectSpec     `json:"spec"`
	Status   ProjectStatus   `json:"status"`
}

func (p *MLRunProject) GetLastJobID() string {
	// MLRun doesn't have jobs associated with projects
	// This is a placeholder to satisfy the CreateProjectResponse interface
	return ""
}

type ProjectMetadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	CreatedAt   string            `json:"created,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ProjectSpec struct {
	Description string `json:"description,omitempty"`
}

type ProjectStatus struct {
	State common.JobState `json:"state,omitempty"`
}

type MlrunError struct {
	Detail string `json:"detail"`
}

func (p *MLRunProject) GetConfig() *platform.ProjectConfig {
	updateAt := common.ParseTimeFromTimestamp(p.Metadata.CreatedAt)
	return &platform.ProjectConfig{
		Meta: platform.ProjectMeta{
			Name:        p.Metadata.Name,
			Annotations: p.Metadata.Annotations,
			Labels:      p.Metadata.Labels,
		},
		Spec: platform.ProjectSpec{},
		Status: platform.ProjectStatus{
			UpdatedAt: &updateAt,
		},
	}
}

func NewProjectFromProjectConfig(projectConfig *platform.ProjectConfig) (MLRunProject, error) {
	if projectConfig == nil {
		return MLRunProject{}, errors.New("ProjectConfig is nil")
	}

	return MLRunProject{
		Metadata: ProjectMetadata{
			Name:        projectConfig.Meta.Name,
			Namespace:   projectConfig.Meta.Namespace,
			Labels:      projectConfig.Meta.Labels,
			Annotations: projectConfig.Meta.Annotations,
		},
		Spec: ProjectSpec{
			Description: projectConfig.Spec.Description,
		},
	}, nil
}

type MLRunProjectList struct {
	Projects []MLRunProject `json:"projects"`
}

// ToProjectList returns list of MLRunProject
func (mpl *MLRunProjectList) ToProjectList() []platform.Project {
	var projects []platform.Project
	for _, mlrunProject := range mpl.Projects {
		projects = append(projects, &MLRunProject{
			Metadata: mlrunProject.Metadata,
			Spec:     mlrunProject.Spec,
			Status:   mlrunProject.Status,
		})
	}
	return projects
}
