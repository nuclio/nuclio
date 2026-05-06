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

package mlrun

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/platform"
	leaderCommon "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type APIVersion string

const (
	APIVersionV1 APIVersion = "v1"
	APIVersionV2 APIVersion = "v2"
)

type LeaderOps struct {
	logger logger.Logger
	// namespace is used to enrich the MLRun responses, which omit the namespace
	namespace string
}

func NewLeaderOps(parentLogger logger.Logger, namespace string) *LeaderOps {
	return &LeaderOps{
		logger:    parentLogger.GetChild("mlrun"),
		namespace: namespace,
	}
}

func (l *LeaderOps) GenerateProjectRequestBody(projectConfig *platform.ProjectConfig) ([]byte, error) {
	project, err := NewProjectFromProjectConfig(projectConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create project from project config")
	}
	return json.Marshal(project)
}

func (l *LeaderOps) GenerateProjectDeletionRequestBody(projectName string) ([]byte, error) {
	return json.Marshal(MLRunProject{
		Metadata: ProjectMetadata{
			Name: projectName,
		},
	})
}

func (l *LeaderOps) ResolveCreateProjectResponse(ctx context.Context, body []byte) (leaderCommon.CreateProjectResponse, error) {
	project := MLRunProject{}
	if err := json.Unmarshal(body, &project); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal response body")
	}

	l.logger.DebugWithCtx(ctx,
		"Successfully sent create project request to leader",
		"project name", project.Metadata.Name)
	return &project, nil
}

func (l *LeaderOps) ResolveGetProjectResponse(_ bool, body []byte) ([]platform.Project, error) {
	var projects MLRunProjectList
	if err := json.Unmarshal(body, &projects); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal response body")
	}

	return projects.ToProjectList(l.namespace), nil
}

func (l *LeaderOps) ParseJobStatusResponse(_ context.Context, _ []byte) (leaderCommon.JobResponse, bool) {
	// MLRun does not have async job handling, so this is a placeholder
	return nil, false
}

func (l *LeaderOps) GenerateCreateProjectRequestURL(apiAddress string) string {
	return fmt.Sprintf("%s/%s/%s", apiAddress, APIVersionV1, "projects")
}

func (l *LeaderOps) HandleCreateResponseErr(ctx context.Context, responseBody []byte, response *http.Response, err error) error {
	// Try to parse MLRun error response
	var mlrunError MlrunError

	// try peek at error response
	if unmarshalErr := json.Unmarshal(responseBody, &mlrunError); unmarshalErr == nil {
		l.logger.ErrorWithCtx(ctx,
			"Create project has failed",
			"err", err,
			"responseError", mlrunError)
		if response == nil {
			return errors.New("Failed to get response from leader, response is nil")
		}
		return nuclio.GetByStatusCode(response.StatusCode)(mlrunError.Detail)
	}
	return errors.Wrap(err, "Failed to send request to leader")
}

func (l *LeaderOps) GetJobIdUrl(_, _ string) string {
	// MLRun does not have async job handling, so this is a placeholder
	return ""
}

func (l *LeaderOps) IsJobCompleted(_ context.Context, _ leaderCommon.JobResponse, _ string) error {
	// MLRun does not have async job handling, so this is a placeholder
	return nil
}

func (l *LeaderOps) GenerateUpdateProjectRequestURL(apiAddress, projectName string) string {
	return l.projectRequestURL(apiAddress, projectName, APIVersionV1)
}

func (l *LeaderOps) GetDeleteExpectedStatusCode() int {
	return http.StatusNoContent
}

func (l *LeaderOps) GetDeleteStrategyHeaderName() string {
	return "x-mlrun-deletion-strategy"
}

func (l *LeaderOps) GenerateGetProjectsRequestURL(apiAddress, projectName string) string {
	url := fmt.Sprintf("%s/%s/projects", apiAddress, APIVersionV1)
	if projectName != "" {
		url += fmt.Sprintf("/%s", projectName)
	}
	return url
}

func (l *LeaderOps) GenerateGetUpdatedAfterRequestURL(apiAddress string) string {
	// TODO - for now there is no filter addition to the URL, should be added when MLRun supports updated_at
	return fmt.Sprintf("%s/%s", apiAddress, "projects")
}

func (l *LeaderOps) GenerateDeleteProjectRequestURL(apiAddress, projectName string) string {
	return l.projectRequestURL(apiAddress, projectName, APIVersionV2)
}

// EvaluateLeaderRequest determines the 2PC phase purely from the request labels and
// validates it against the current CRD state.
// Phase detection rules (labels on the incoming request):
//   - sync-status=creating                         → Provision
//   - sync-status=online,   no current-op-id       → Commit
//   - sync-status=deleting, current-op-id present  → Mark-delete
//   - sync-status=online,   current-op-id present  → Spec update
//   - no sync-status (only op_id)                  → Final-delete
func (l *LeaderOps) EvaluateLeaderRequest(_ context.Context, labels map[string]string, existing platform.Project) (bool, error) {
	syncStatus := labels[leaderCommon.MLRunLabelKeySyncStatus]
	currentOpID := labels[leaderCommon.MLRunLabelKeyCurrentOpID]

	switch {
	case syncStatus == leaderCommon.MLRunSyncStatusCreating:
		return l.validateProvision(labels, existing)
	case syncStatus == leaderCommon.MLRunSyncStatusOnline && currentOpID == "":
		return l.validateCommit(labels, existing)
	case syncStatus == leaderCommon.MLRunSyncStatusDeleting && currentOpID != "":
		return l.validateMarkDelete(labels, existing)
	case syncStatus == leaderCommon.MLRunSyncStatusOnline && currentOpID != "":
		return l.validateSpecUpdate(labels, existing)
	case syncStatus == "":
		return l.validateFinalDelete(labels, existing)
	default:
		return false, nuclio.GetByStatusCode(http.StatusBadRequest)(
			fmt.Sprintf("Unrecognised 2PC labels: sync-status=%q current-op-id=%q",
				syncStatus, currentOpID))
	}
}

func (l *LeaderOps) ShouldWaitForCreateCompletion() bool { return false }

func (l *LeaderOps) GetJobStatusRequestCookies(_ *platformconfig.Config) []*http.Cookie { return nil }

func (l *LeaderOps) GetJobRequestFilter(_ *time.Time) string { return "" }

func (l *LeaderOps) GetAuthSessionCookie(_ auth.Session) *http.Cookie { return nil }

func (l *LeaderOps) AddAuthSessionHeaders(headers map[string]string, authSession auth.Session) {
	headers["authorization"] = authSession.CompileAuthorizationHeader()
}

func (l *LeaderOps) projectRequestURL(apiAddress, projectName string, version APIVersion) string {
	return fmt.Sprintf("%s/%s/%s/%s", apiAddress, version, "projects", projectName)
}

// validateProvision validates the Provision step (sync-status=creating in request labels).
// Returns (true, nil)  – no CRD yet, caller should create.
// Returns (false, nil) – same op_id already stored, idempotent replay.
func (l *LeaderOps) validateProvision(labels map[string]string, existing platform.Project) (bool, error) {
	requestedOpID, err := l.requireLabel(labels, leaderCommon.MLRunLabelKeyOpID, "provision")
	if err != nil {
		return false, err
	}

	if existing == nil {
		// no CRD yet — caller should create
		return true, nil
	}

	storedOpID, effectiveStatus := l.extractCRDState(existing)

	if storedOpID == requestedOpID {
		// idempotent: caller returns the existing project without re-creating
		return false, nil
	}

	if !l.isOpIDOrdered(requestedOpID, storedOpID) {
		return false, nuclio.GetByStatusCode(http.StatusConflict)(
			fmt.Sprintf("Provision rejected: op_id %q is older than stored op_id %q (replay protection)",
				requestedOpID, storedOpID))
	}

	if err := l.requireSyncStatus(effectiveStatus, leaderCommon.MLRunSyncStatusCreating, "Provision"); err != nil {
		return false, err
	}

	return false, nuclio.GetByStatusCode(http.StatusConflict)(
		fmt.Sprintf("Provision rejected: op_id mismatch (requested %q, stored %q)",
			requestedOpID, storedOpID))
}

// validateCommit validates the Commit step: creating → online.
// Returns (true, nil) on success; caller should update the CRD.
func (l *LeaderOps) validateCommit(labels map[string]string, existing platform.Project) (bool, error) {
	requestedOpID, err := l.requireLabel(labels, leaderCommon.MLRunLabelKeyOpID, "commit")
	if err != nil {
		return false, err
	}

	if err := l.requireExistingProject(existing, "Commit"); err != nil {
		return false, err
	}

	storedOpID, effectiveStatus := l.extractCRDState(existing)

	if err := l.requireSyncStatus(effectiveStatus, leaderCommon.MLRunSyncStatusCreating, "Commit"); err != nil {
		return false, err
	}

	if err := l.requireOpIDMatch(requestedOpID, storedOpID, "Commit"); err != nil {
		return false, err
	}
	return true, nil
}

// validateMarkDelete validates the Mark-delete step: online → deleting.
// The current-op-id label acts as a CAS key against the stored op_id.
// Returns (true, nil) on success; caller should update the CRD.
func (l *LeaderOps) validateMarkDelete(labels map[string]string, existing platform.Project) (bool, error) {
	if err := l.requireExistingProject(existing, "Mark-delete"); err != nil {
		return false, err
	}

	storedOpID, effectiveStatus := l.extractCRDState(existing)

	if err := l.requireSyncStatus(effectiveStatus, leaderCommon.MLRunSyncStatusOnline, "Mark-delete"); err != nil {
		return false, err
	}

	if err := l.requireOpIDMatch(labels[leaderCommon.MLRunLabelKeyCurrentOpID], storedOpID, "Mark-delete"); err != nil {
		return false, err
	}

	if err := l.requireNewerOpID(labels[leaderCommon.MLRunLabelKeyOpID], storedOpID, "Mark-delete"); err != nil {
		return false, err
	}
	return true, nil
}

// validateSpecUpdate validates an in-place spec update: advances op_id while staying online.
// The current-op-id label acts as a CAS key; op_id carries the replacement value.
// Returns (true, nil) on success; caller should update the CRD.
func (l *LeaderOps) validateSpecUpdate(labels map[string]string, existing platform.Project) (bool, error) {
	if err := l.requireExistingProject(existing, "Update"); err != nil {
		return false, err
	}

	storedOpID, effectiveStatus := l.extractCRDState(existing)

	if err := l.requireSyncStatus(effectiveStatus, leaderCommon.MLRunSyncStatusOnline, "Update"); err != nil {
		return false, err
	}

	if err := l.requireOpIDMatch(labels[leaderCommon.MLRunLabelKeyCurrentOpID], storedOpID, "Update"); err != nil {
		return false, err
	}

	if err := l.requireNewerOpID(labels[leaderCommon.MLRunLabelKeyOpID], storedOpID, "Update"); err != nil {
		return false, err
	}
	return true, nil
}

// validateFinalDelete validates the Final-delete step.
// Returns (false, nil) when the CRD is already gone (idempotent).
// Returns (true, nil)  on success; caller should delete the CRD.
func (l *LeaderOps) validateFinalDelete(labels map[string]string, existing platform.Project) (bool, error) {
	requestedOpID, err := l.requireLabel(labels, leaderCommon.MLRunLabelKeyOpID, "final-delete")
	if err != nil {
		return false, err
	}

	if existing == nil {
		return false, nil // idempotent: CRD already gone
	}

	storedOpID, effectiveStatus := l.extractCRDState(existing)

	if err := l.requireSyncStatus(effectiveStatus, leaderCommon.MLRunSyncStatusDeleting, "Final-delete"); err != nil {
		return false, err
	}

	if err := l.requireOpIDMatch(requestedOpID, storedOpID, "Final-delete"); err != nil {
		return false, err
	}
	return true, nil
}

// requireLabel returns the value of key from labels, or a 400 error if it is absent.
func (l *LeaderOps) requireLabel(labels map[string]string, key, operation string) (string, error) {
	value := labels[key]
	if value == "" {
		return "", nuclio.GetByStatusCode(http.StatusBadRequest)(
			fmt.Sprintf("Missing %q label for 2PC %s", key, operation))
	}
	return value, nil
}

// requireExistingProject returns a 412 error when no CRD is present.
func (l *LeaderOps) requireExistingProject(existing platform.Project, operation string) error {
	if existing == nil {
		return nuclio.GetByStatusCode(http.StatusPreconditionFailed)(
			fmt.Sprintf("project CRD not found [%s]", operation))
	}
	return nil
}

// extractCRDState returns the stored op_id and the effective sync-status from an existing CRD.
func (l *LeaderOps) extractCRDState(existing platform.Project) (storedOpID, syncStatus string) {
	labels := existing.GetConfig().Meta.Labels
	return labels[leaderCommon.MLRunLabelKeyOpID], l.resolveSyncStatus(labels)
}

// requireSyncStatus returns a 412 error when the CRD's effective status does not match expected.
func (l *LeaderOps) requireSyncStatus(effectiveStatus, expectedStatus, operation string) error {
	if effectiveStatus != expectedStatus {
		return nuclio.GetByStatusCode(http.StatusPreconditionFailed)(
			fmt.Sprintf("project is in %q state, expected %q [%s]",
				effectiveStatus, expectedStatus, operation))
	}
	return nil
}

// requireOpIDMatch returns a 409 error when the stored op_id does not equal the requested one.
func (l *LeaderOps) requireOpIDMatch(requestedOpID, storedOpID, operation string) error {
	if storedOpID != requestedOpID {
		return nuclio.GetByStatusCode(http.StatusConflict)(
			fmt.Sprintf("op_id mismatch (requested %q, stored %q) [%s]",
				requestedOpID, storedOpID, operation))
	}
	return nil
}

// requireNewerOpID returns a 409 error when newOpID is present but not strictly newer than storedOpID.
func (l *LeaderOps) requireNewerOpID(newOpID, storedOpID, operation string) error {
	if newOpID != "" && !l.isOpIDOrdered(newOpID, storedOpID) {
		return nuclio.GetByStatusCode(http.StatusConflict)(
			fmt.Sprintf("new op_id %q is not newer than stored op_id %q, replay protection [%s]",
				newOpID, storedOpID, operation))
	}
	return nil
}

// resolveSyncStatus returns the effective sync-status label value, defaulting to
// MLRunSyncStatusOnline for CRDs that pre-date 2PC introduction (backwards compatibility).
func (l *LeaderOps) resolveSyncStatus(labels map[string]string) string {
	if status, exists := labels[leaderCommon.MLRunLabelKeySyncStatus]; exists {
		return status
	}
	return leaderCommon.MLRunSyncStatusOnline
}

// isOpIDOrdered returns true when newOpID is strictly newer than storedOpID.
// UUIDv7 encodes a millisecond-precision timestamp in the most-significant bits,
// making lexicographic string comparison equivalent to chronological ordering.
func (l *LeaderOps) isOpIDOrdered(newOpID, storedOpID string) bool {
	return newOpID > storedOpID
}
