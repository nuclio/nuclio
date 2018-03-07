/*
Copyright 2017 The Nuclio Authors.

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

package platform

import (
	"github.com/nuclio/logger"
)

type Project interface {

	// GetID returns the project ID
	GetID() ProjectID

	// GetName returns the project name
	GetName() string

	// GetDescription returns the project description
	GetDescription() string
}

type AbstractProject struct {
	Logger   logger.Logger
	Platform Platform
	ProjectConfig ProjectConfig
}

func NewAbstractProject(parentLogger logger.Logger,
	parentPlatform Platform,
	ProjectConfig ProjectConfig) (*AbstractProject, error) {

	return &AbstractProject{
		Logger:   parentLogger.GetChild("project"),
		Platform: parentPlatform,
		ProjectConfig: ProjectConfig,
	}, nil
}

// GetID returns the project ID
func (ap *AbstractProject) GetID() ProjectID {
	return ap.ProjectConfig.ID
}

// GetName returns the project name
func (ap *AbstractProject) GetName() string {
	return ap.ProjectConfig.Name
}

// GetDescription returns the project description
func (ap *AbstractProject) GetDescription() string {
	return ap.ProjectConfig.Description
}
