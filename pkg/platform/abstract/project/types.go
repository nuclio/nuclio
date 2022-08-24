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
package project

import (
	"context"

	"github.com/nuclio/nuclio/pkg/platform"
)

type Client interface {

	// Initialize client
	Initialize() error

	// Create a new project
	Create(context.Context, *platform.CreateProjectOptions) (platform.Project, error)

	// Update a project
	Update(context.Context, *platform.UpdateProjectOptions) (platform.Project, error)

	// Delete a project
	Delete(context.Context, *platform.DeleteProjectOptions) error

	// Get projects
	Get(context.Context, *platform.GetProjectsOptions) ([]platform.Project, error)
}
