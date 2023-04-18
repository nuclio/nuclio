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

package leader

import (
	"context"
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
)

type Client interface {

	// Get delegate the project get to leader
	Get(context.Context, *platform.GetProjectsOptions) ([]platform.Project, error)

	// Create delegates project creation to leader
	Create(context.Context, *platform.CreateProjectOptions) error

	// Update delegates project update to leader
	Update(context.Context, *platform.UpdateProjectOptions) error

	// Delete delegates project deletion to leader
	Delete(context.Context, *platform.DeleteProjectOptions) error

	// GetUpdatedAfter gets all projects from the leader that updated after the given time (to get all, pass nil time)
	GetUpdatedAfter(context.Context, *time.Time) ([]platform.Project, error)
}
