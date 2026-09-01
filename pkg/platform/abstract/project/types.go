/*
Copyright 2023 The Nuclio Authors.

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

	// 2PC project operations: the dedicated /api/v1/follower/projects/* surface. Only
	// internalc/kube.Client (when Oris is the configured leader) implements these for real,
	// including the CAS validation against the existing CRD state, which is this client's
	// own private concern.

	// PrepareCreate prepares a project follower creation, returning the state of the follower
	PrepareCreate(context.Context, *platform.PrepareCreateProjectOptions) (*platform.Project2PCState, error)

	// CommitCreate commits a project follower creation, returning the state of the follower
	CommitCreate(context.Context, *platform.CommitCreateProjectOptions) (*platform.Project2PCState, error)

	// CommitUpdate updates a project follower, returning the state of the follower
	CommitUpdate(context.Context, *platform.CommitUpdateProjectOptions) (*platform.Project2PCState, error)

	// PrepareDelete prepares a project follower deletion, returning the state of the follower
	PrepareDelete(context.Context, *platform.PrepareDeleteProjectOptions) (*platform.Project2PCState, error)

	// CommitDelete commits a project follower deletion, returning the state of the follower
	CommitDelete(context.Context, *platform.CommitDeleteProjectOptions) (*platform.Project2PCState, error)

	// List lists the states of project followers
	List(context.Context, *platform.ListProjectStatesOptions) (*platform.Project2PCStatesPage, error)
}
