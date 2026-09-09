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

package kube

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project"
	leaderCommon "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	nuclioclient "github.com/nuclio/nuclio/pkg/platform/kube/clients/nuclio"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Client struct {
	Logger   logger.Logger
	platform platform.Platform
	consumer *nuclioclient.Consumer
}

func NewClient(parentLogger logger.Logger,
	platformInstance platform.Platform,
	consumer *nuclioclient.Consumer) (project.Client, error) {
	newClient := &Client{
		Logger:   parentLogger.GetChild("projects-kube"),
		consumer: consumer,
		platform: platformInstance,
	}

	return newClient, nil
}

func (c *Client) Initialize() error {
	return nil
}

func (c *Client) Get(ctx context.Context, getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	var platformProjects []platform.Project
	var projects []nuclioio.NuclioProject

	// if identifier specified, we need to get a single NuclioProject
	if getProjectsOptions.Meta.Name != "" {

		// get specific NuclioProject CR
		projectInstance, err := c.consumer.NuclioClientSet.GetNuclioProject(ctx,
			getProjectsOptions.Meta.Namespace,
			getProjectsOptions.Meta.Name)

		if err != nil {

			// if we didn't find the NuclioProject, return an empty slice
			if apierrors.IsNotFound(err) {
				return platformProjects, nil
			}

			return nil, errors.Wrap(err, "Failed to get a project")
		}

		projects = append(projects, *projectInstance)

	} else {

		projectInstanceList, err := c.consumer.NuclioClientSet.ListNuclioProjects(ctx,
			getProjectsOptions.Meta.Namespace,
			metav1.ListOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "Failed to list projects")
		}

		// convert []NuclioProject to []*NuclioProject
		projects = projectInstanceList.Items
	}

	// convert each nuclioio.NuclioProject -> platform.Project
	for projectInstanceIndex := 0; projectInstanceIndex < len(projects); projectInstanceIndex++ {
		projectInstance := projects[projectInstanceIndex]

		newProject, err := c.nuclioProjectToPlatformProject(&projectInstance)
		if err != nil {
			return nil, err
		}

		platformProjects = append(platformProjects, newProject)
	}

	return platformProjects, nil
}

func (c *Client) Create(ctx context.Context, createProjectOptions *platform.CreateProjectOptions) (platform.Project, error) {
	newProject := nuclioio.NuclioProject{}
	c.platformProjectToProject(createProjectOptions.ProjectConfig, &newProject)

	nuclioProject, err := c.consumer.NuclioClientSet.CreateNuclioProject(ctx, newProject.Namespace, &newProject)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil, nuclio.WrapErrConflict(err)
		}
		return nil, errors.Wrap(err, "Failed to create nuclio project")
	}

	return c.nuclioProjectToPlatformProject(nuclioProject)
}

func (c *Client) Update(ctx context.Context, updateProjectOptions *platform.UpdateProjectOptions) (platform.Project, error) {
	projectInstance, err := c.consumer.NuclioClientSet.GetNuclioProject(ctx,
		updateProjectOptions.ProjectConfig.Meta.Namespace,
		updateProjectOptions.ProjectConfig.Meta.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nuclio.WrapErrNotFound(err)
		}
		return nil, errors.Wrap(err, "Failed to get a project")
	}

	updatedProject := nuclioio.NuclioProject{}
	c.platformProjectToProject(&updateProjectOptions.ProjectConfig, &updatedProject)
	projectInstance.Spec = updatedProject.Spec
	projectInstance.Annotations = updatedProject.Annotations
	projectInstance.Labels = updatedProject.Labels
	projectInstance.Status = updatedProject.Status
	now := time.Now()
	projectInstance.Status.UpdatedAt = &now

	nuclioProject, err := c.consumer.NuclioClientSet.UpdateNuclioProject(ctx, projectInstance.Namespace, projectInstance)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to update nuclio project")
	}

	return c.nuclioProjectToPlatformProject(nuclioProject)
}

func (c *Client) Delete(ctx context.Context, deleteProjectOptions *platform.DeleteProjectOptions) error {
	if err := c.consumer.NuclioClientSet.DeleteNuclioProject(ctx,
		deleteProjectOptions.Meta.Namespace,
		deleteProjectOptions.Meta.Name,
		metav1.DeleteOptions{}); err != nil {

		if apierrors.IsNotFound(err) {
			return nuclio.NewErrNotFound(fmt.Sprintf("Project %s not found", deleteProjectOptions.Meta.Name))
		}
		return errors.Wrapf(err,
			"Failed to delete project %s from namespace %s",
			deleteProjectOptions.Meta.Name,
			deleteProjectOptions.Meta.Namespace)
	}

	if deleteProjectOptions.WaitForResourcesDeletionCompletion {
		return c.platform.WaitForProjectResourcesDeletion(ctx,
			&deleteProjectOptions.Meta,
			deleteProjectOptions.WaitForResourcesDeletionCompletionDuration)
	}

	return nil
}

// Follower operations: the dedicated /api/follower/projects/* surface is only ever
// reachable when Oris is the configured leader. This is the only client that implements
// these for real -- external.Client and the local platform's client are permanently
// unsupported.

// PrepareCreate provisions a new project CRD (2PC step 1).
func (c *Client) PrepareCreate(ctx context.Context,
	options *platform.PrepareCreateProjectOptions) (*platform.Project2PCState, error) {
	name := options.ProjectConfig.Meta.Name
	namespace := options.ProjectConfig.Meta.Namespace
	existingProject, err := c.getProject(ctx, name, namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to prepare create")
	}

	if existingProject == nil {
		if err := c.writeFollowerProject(ctx, false, options.ProjectConfig, options.OpID, leaderCommon.OrisSyncStatusCreating); err != nil {
			return nil, errors.Wrap(err, "Failed to prepare create")
		}
		c.Logger.DebugWithCtx(ctx, "completed successfully", "name", name, "opID", options.OpID)
		return &platform.Project2PCState{Name: name, OpID: options.OpID, SyncStatus: string(leaderCommon.OrisSyncStatusCreating)}, nil
	}

	currentOpID, currentStatus := c.extractProjectLabels(existingProject)

	// Idempotency: the project already exists with this opID
	if leaderCommon.IsOpIDEqual(currentOpID, options.OpID) {
		c.Logger.DebugWithCtx(ctx, "opID already applied, considered as completed successfully", "name", name, "opID", options.OpID)
		return &platform.Project2PCState{Name: name, OpID: currentOpID, SyncStatus: string(currentStatus)}, nil
	}

	if err := leaderCommon.RequireOpIDOrdered(options.OpID, currentOpID); err != nil {
		return nil, nuclio.GetByStatusCode(http.StatusConflict)(
			fmt.Sprintf("opID ordering check failed for project %q: %s", name, err.Error()))
	}

	// The incoming opID is newer. If the project is still "creating" this is a recovery scenario:
	// the leader abandoned the previous provision and is starting fresh with a new opID.
	// Allow the overwrite so the existingProject is not stuck.
	if currentStatus == leaderCommon.OrisSyncStatusCreating {
		c.Logger.DebugWithCtx(ctx, "overwriting abandoned provision", "name", name, "opID", options.OpID, "currentOpID", currentOpID)
		if err := c.writeFollowerProject(ctx, true, options.ProjectConfig, options.OpID, leaderCommon.OrisSyncStatusCreating); err != nil {
			return nil, errors.Wrap(err, "Failed to prepare create")
		}
		c.Logger.DebugWithCtx(ctx, "completed successfully", "name", name, "opID", options.OpID)
		return &platform.Project2PCState{Name: name, OpID: options.OpID, SyncStatus: string(leaderCommon.OrisSyncStatusCreating)}, nil
	}

	// The opID is newer, but the project isn't in "creating" state — it's already
	// provisioned (online or deleting) and cannot be re-provisioned.
	return nil, nuclio.GetByStatusCode(http.StatusConflict)(
		fmt.Sprintf("opID is newer than stored opID, but project is not in creating state (opID %q, storedOpID %q, status %q)",
			options.OpID, currentOpID, currentStatus))
}

// CommitCreate activates a provisioned project (2PC step 2): flips its sync-status from creating to online.
func (c *Client) CommitCreate(ctx context.Context,
	options *platform.CommitCreateProjectOptions) (*platform.Project2PCState, error) {
	name := options.Meta.Name
	existingProject, err := c.getProject(ctx, name, options.Meta.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to commit create")
	}
	if existingProject == nil {
		return nil, nuclio.GetByStatusCode(http.StatusPreconditionFailed)(
			fmt.Sprintf("project does not exist, prepare create must run first (project %q)", name))
	}

	currentOpID, currentStatus := c.extractProjectLabels(existingProject)
	if err := leaderCommon.RequireOpIDMatch(options.OpID, currentOpID); err != nil {
		return nil, nuclio.GetByStatusCode(http.StatusConflict)(err.Error())
	}

	switch currentStatus {
	case leaderCommon.OrisSyncStatusOnline:
		// Idempotency: already online with this opID — the commit was already applied.
		c.Logger.DebugWithCtx(ctx, "project status is already online, considered as completed successfully",
			"name", name, "opID", options.OpID, "current opID", currentOpID)
		return &platform.Project2PCState{Name: name, OpID: currentOpID, SyncStatus: string(currentStatus)}, nil
	case leaderCommon.OrisSyncStatusCreating:
		// Expected state — fall through and complete the commit.
		c.Logger.DebugWithCtx(ctx, "project status is creating, committing to online", "name", name, "opID", options.OpID)
	default:
		return nil, unexpectedStateError(http.StatusPreconditionFailed, name, currentStatus, leaderCommon.OrisSyncStatusCreating)
	}

	if err := c.updateFollowerProjectLabels(ctx, existingProject, options.OpID, leaderCommon.OrisSyncStatusOnline); err != nil {
		return nil, errors.Wrap(err, "Failed to commit create")
	}
	c.Logger.DebugWithCtx(ctx, "completed successfully", "name", name, "opID", options.OpID)
	return &platform.Project2PCState{Name: name, OpID: options.OpID, SyncStatus: string(leaderCommon.OrisSyncStatusOnline)}, nil
}

// CommitUpdate applies a common-set update to an online project: CAS against PrevOpID,
// require the new OpID to be strictly newer, then advance the CRD's spec/labels.
func (c *Client) CommitUpdate(ctx context.Context,
	options *platform.CommitUpdateProjectOptions) (*platform.Project2PCState, error) {
	name := options.ProjectConfig.Meta.Name
	existingProject, err := c.getProject(ctx, name, options.ProjectConfig.Meta.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to commit update")
	}
	if existingProject == nil {
		return nil, nuclio.NewErrNotFound(fmt.Sprintf("Update rejected: project not found (project %q)", name))
	}

	currentOpID, currentStatus := c.extractProjectLabels(existingProject)

	// "must be online" precondition by design
	if currentStatus != leaderCommon.OrisSyncStatusOnline {
		return nil, unexpectedStateError(http.StatusPreconditionFailed, name, currentStatus, leaderCommon.OrisSyncStatusOnline)
	}

	// Idempotency: already applied — must be checked before CAS, since after a successful
	// update the stored opID has advanced past the request's PrevOpID.
	if leaderCommon.IsOpIDEqual(currentOpID, options.OpID) {
		c.Logger.DebugWithCtx(ctx, "opID already applied, considered as completed successfully",
			"name", name, "opID", options.OpID, "current opID", currentOpID)
		return &platform.Project2PCState{Name: name, OpID: currentOpID, SyncStatus: string(currentStatus)}, nil
	}

	if err := leaderCommon.RequireCASMatch(options.PrevOpID, currentOpID); err != nil {
		return nil, errors.Wrap(err, "Update CAS check failed")
	}
	if err := leaderCommon.RequireOpIDOrdered(options.OpID, currentOpID); err != nil {
		return nil, nuclio.GetByStatusCode(http.StatusConflict)(fmt.Sprintf("opID ordering check failed for project %q: %s", name, err.Error()))
	}

	if err := c.writeFollowerProject(ctx, true, options.ProjectConfig, options.OpID, leaderCommon.OrisSyncStatusOnline); err != nil {
		return nil, errors.Wrap(err, "Failed to commit update")
	}
	c.Logger.DebugWithCtx(ctx, "completed successfully", "name", name, "opID", options.OpID)
	return &platform.Project2PCState{Name: name, OpID: options.OpID, SyncStatus: string(leaderCommon.OrisSyncStatusOnline)}, nil
}

// PrepareDelete marks a project deleting (2PC delete step 1).
func (c *Client) PrepareDelete(ctx context.Context,
	options *platform.PrepareDeleteProjectOptions) (*platform.Project2PCState, error) {
	name := options.Meta.Name
	existingProject, err := c.getProject(ctx, name, options.Meta.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to prepare delete")
	}

	// Idempotency: the CRD does not exist - no need to mark it deleting, the commit delete will be a no-op.
	if existingProject == nil {
		c.Logger.DebugWithCtx(ctx, "project does not exist, considered as completed successfully", "name", name, "opID", options.OpID)
		return &platform.Project2PCState{Name: name, OpID: options.OpID, SyncStatus: string(leaderCommon.OrisSyncStatusDeleting)}, nil
	}

	currentOpID, currentStatus := c.extractProjectLabels(existingProject)

	// Idempotency: this exact mark-delete already applied. Only a prior, successful call to
	// this function could have stamped this opID, so the status is guaranteed to be deleting.
	if leaderCommon.IsOpIDEqual(currentOpID, options.OpID) {
		c.Logger.DebugWithCtx(ctx, "opID with deleting status already applied, considered as completed successfully",
			"name", name, "opID", options.OpID, "current opID", currentOpID)
		return &platform.Project2PCState{Name: name, OpID: currentOpID, SyncStatus: string(currentStatus)}, nil
	}

	switch currentStatus {
	case leaderCommon.OrisSyncStatusDeleting:
		// Conflict: already deleting under a different opID — a different delete is in
		// progress, must not be silently overwritten (mlrun's validateMarkDelete rejects the
		// same way, rather than treating any in-progress delete as idempotent).
		return nil, nuclio.GetByStatusCode(http.StatusConflict)(
			fmt.Sprintf("project status is already deleting under a different opID (current opID %q, opID %q)", currentOpID, options.OpID))
	case leaderCommon.OrisSyncStatusOnline:
		// Expected state — fall through and continue below.
		c.Logger.DebugWithCtx(ctx, "project status is online, marking deleting", "name", name, "opID", options.OpID)
	default:
		return nil, unexpectedStateError(http.StatusPreconditionFailed, name, currentStatus, leaderCommon.OrisSyncStatusOnline)
	}

	if err := leaderCommon.RequireCASMatch(options.PrevOpID, currentOpID); err != nil {
		return nil, errors.Wrap(err, "CAS check failed")
	}
	if err := leaderCommon.RequireOpIDOrdered(options.OpID, currentOpID); err != nil {
		return nil, nuclio.GetByStatusCode(http.StatusConflict)(fmt.Sprintf("opID ordering check failed for project %q: %s", name, err.Error()))
	}

	if err := c.updateFollowerProjectLabels(ctx, existingProject, options.OpID, leaderCommon.OrisSyncStatusDeleting); err != nil {
		return nil, errors.Wrap(err, "Failed to prepare delete")
	}
	c.Logger.DebugWithCtx(ctx, "completed successfully", "name", name, "opID", options.OpID)
	return &platform.Project2PCState{Name: name, OpID: options.OpID, SyncStatus: string(leaderCommon.OrisSyncStatusDeleting)}, nil
}

// CommitDelete purges the project CRD (2PC delete step 2).
func (c *Client) CommitDelete(ctx context.Context,
	options *platform.CommitDeleteProjectOptions) (*platform.Project2PCState, error) {
	name := options.Meta.Name
	existingProject, err := c.getProject(ctx, name, options.Meta.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to commit delete")
	}

	// Idempotency: already gone, a previous call already deleted it.
	if existingProject == nil {
		c.Logger.DebugWithCtx(ctx, "project does not exist, considered as completed successfully", "name", name, "opID", options.OpID)
		return &platform.Project2PCState{Name: name, OpID: options.OpID}, nil
	}

	currentOpID, currentStatus := c.extractProjectLabels(existingProject)
	if currentStatus != leaderCommon.OrisSyncStatusDeleting {
		return nil, unexpectedStateError(http.StatusConflict, name, currentStatus, leaderCommon.OrisSyncStatusDeleting)
	}
	if err := leaderCommon.RequireOpIDMatch(options.OpID, currentOpID); err != nil {
		return nil, nuclio.GetByStatusCode(http.StatusConflict)(err.Error())
	}

	if err := c.Delete(ctx, &platform.DeleteProjectOptions{Meta: options.Meta}); err != nil {
		return nil, errors.Wrap(err, "Failed to delete project")
	}

	c.Logger.DebugWithCtx(ctx, "completed successfully", "name", name, "opID", options.OpID)
	return &platform.Project2PCState{Name: name, OpID: options.OpID}, nil
}

// List lists this follower's project states for the leader's reconciliation sweep.
func (c *Client) List(ctx context.Context,
	options *platform.ListProjectStatesOptions) (*platform.Project2PCStatesPage, error) {
	projects, err := c.Get(ctx, &platform.GetProjectsOptions{Meta: platform.ProjectMeta{Namespace: options.Namespace}})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list projects")
	}

	states := make([]*platform.Project2PCState, 0, len(projects))
	for _, proj := range projects {
		config := proj.GetConfig()
		// Skip projects that haven't changed since the cutoff — nothing for the leader to reconcile.
		if options.UpdatedAfter != nil && config.Status.UpdatedAt != nil &&
			!config.Status.UpdatedAt.After(*options.UpdatedAfter) {
			continue
		}
		currentOpID, currentStatus := c.extractProjectLabels(proj)
		states = append(states, &platform.Project2PCState{
			Name:       config.Meta.Name,
			OpID:       currentOpID,
			SyncStatus: string(currentStatus),
		})
	}

	// Get returns projects in whatever order the k8s client/cache happens to produce, which is
	// not stable across calls. The keyset pagination below requires a deterministic order to
	// page through every project exactly once, so impose one here by Name.
	sort.Slice(states, func(i, j int) bool { return states[i].Name < states[j].Name })

	// Cursor is the Name of the last project returned in the previous page. The leader's
	// reconciliation sweep pages through a follower's entire project set by repeatedly calling
	// List and feeding the previous response's NextCursor back in as Cursor, so it never has to
	// hold the whole set in memory at once (see orca's HTTPFollowerClient.ListStates).
	if options.Cursor != "" {
		remaining := states[:0]
		for _, state := range states {
			if state.Name > options.Cursor {
				remaining = append(remaining, state)
			}
		}
		states = remaining
	}

	var nextCursor string
	if options.Limit > 0 && len(states) > options.Limit {
		states = states[:options.Limit]
		nextCursor = states[len(states)-1].Name
	}

	return &platform.Project2PCStatesPage{States: states, NextCursor: nextCursor}, nil
}

func (c *Client) platformProjectToProject(platformProject *platform.ProjectConfig, project *nuclioio.NuclioProject) {
	project.Name = platformProject.Meta.Name
	project.Namespace = platformProject.Meta.Namespace
	project.Labels = platformProject.Meta.Labels
	project.Annotations = platformProject.Meta.Annotations
	project.Spec = platformProject.Spec
	project.Status = platformProject.Status
}

func (c *Client) nuclioProjectToPlatformProject(nuclioProject *nuclioio.NuclioProject) (platform.Project, error) {
	return platform.NewAbstractProject(c.Logger,
		c.platform,
		platform.ProjectConfig{
			Meta: platform.ProjectMeta{
				Name:        nuclioProject.Name,
				Namespace:   nuclioProject.Namespace,
				Labels:      nuclioProject.Labels,
				Annotations: nuclioProject.Annotations,
			},
			Spec:   nuclioProject.Spec,
			Status: nuclioProject.Status,
		})
}

// getProject get the project by name and namespace.
func (c *Client) getProject(ctx context.Context, name, namespace string) (platform.Project, error) {
	projects, err := c.Get(ctx, &platform.GetProjectsOptions{Meta: platform.ProjectMeta{Name: name, Namespace: namespace}})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to fetch existing project")
	}
	if len(projects) == 0 {
		return nil, nil
	}
	return projects[0], nil
}

// extractProjectLabels reads the opID and oris/sync-status labels off an existing CRD.
func (c *Client) extractProjectLabels(existing platform.Project) (currentOpID string, syncStatus leaderCommon.OrisSyncStatus) {
	labels := existing.GetConfig().Meta.Labels
	currentOpID = labels[leaderCommon.OrisLabelKeyOpID]
	if status, exists := labels[leaderCommon.OrisLabelKeySyncStatus]; exists {
		return currentOpID, leaderCommon.OrisSyncStatus(status)
	}
	// absent sync-status label means the CRD pre-dates this follower surface — treated as
	// "online", the same convention MLRun's evaluator uses for its own legacy labels.
	return currentOpID, leaderCommon.OrisSyncStatusOnline
}

// writeFollowerProject creates or updates the project CRD, stamped with the given opID/sync-status labels.
func (c *Client) writeFollowerProject(ctx context.Context, isUpdate bool,
	projectConfig platform.ProjectConfig, opID string, syncStatus leaderCommon.OrisSyncStatus) error {
	projectConfig.Meta.Labels = c.stampFollowerLabels(projectConfig.Meta.Labels, opID, syncStatus)

	if isUpdate {
		_, err := c.Update(ctx, &platform.UpdateProjectOptions{ProjectConfig: projectConfig})
		return errors.Wrap(err, "Failed to update project")
	}

	// Update always stamps UpdatedAt itself; Create doesn't, so stamp it here too, keeping
	// List's UpdatedAfter reconciliation sweep accurate from the very first write.
	now := time.Now()
	projectConfig.Status.UpdatedAt = &now
	_, err := c.Create(ctx, &platform.CreateProjectOptions{ProjectConfig: &projectConfig})
	return errors.Wrap(err, "Failed to create project")
}

// updateFollowerProjectLabels re-writes an existing project's full config with only its opID/
// sync-status labels advanced, leaving Spec and every other label/annotation untouched.
func (c *Client) updateFollowerProjectLabels(ctx context.Context, existing platform.Project,
	opID string, syncStatus leaderCommon.OrisSyncStatus) error {
	// existing is never nil here: every caller already handles the not-found/idempotent case
	// before reaching this point, so GetConfig() is always safe to dereference.
	projectConfig := *existing.GetConfig()
	projectConfig.Meta.Labels = c.stampFollowerLabels(projectConfig.Meta.Labels, opID, syncStatus)

	// Update replaces Spec/Labels/Annotations/Status wholesale rather than merging, so this must start
	// from the existing CRD's full ProjectConfig, not a labels-only partial one.
	_, err := c.Update(ctx, &platform.UpdateProjectOptions{ProjectConfig: projectConfig})
	return errors.Wrap(err, "Failed to update project")
}

// stampFollowerLabels returns a copy of labels with the oris/* 2PC labels set to opID/
// syncStatus, without mutating the caller's map.
func (c *Client) stampFollowerLabels(labels map[string]string, opID string, syncStatus leaderCommon.OrisSyncStatus) map[string]string {
	stamped := make(map[string]string, len(labels)+2)
	for key, value := range labels {
		stamped[key] = value
	}
	stamped[leaderCommon.OrisLabelKeyOpID] = opID
	stamped[leaderCommon.OrisLabelKeySyncStatus] = string(syncStatus)
	return stamped
}

// unexpectedStateError builds the error returned when a project's currentStatus is not the
// expectedStatus for the operation being attempted.
func unexpectedStateError(statusCode int, name string, currentStatus, expectedStatus leaderCommon.OrisSyncStatus) error {
	return nuclio.GetByStatusCode(statusCode)(
		fmt.Sprintf("project is in unexpected state (project %q, state %q, expected %q)",
			name, currentStatus, expectedStatus))
}
