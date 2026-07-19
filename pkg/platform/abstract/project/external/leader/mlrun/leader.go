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

	"github.com/nuclio/nuclio/pkg/platform"
	leaderCommon "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"
	leaderabstract "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/abstract"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type LeaderOps struct {
	*leaderabstract.LeaderOps
	logger logger.Logger
	// namespace is used to enrich the MLRun responses, which omit the namespace
	namespace             string
	projectSync2PCEnabled bool
}

func NewLeaderOps(parentLogger logger.Logger, namespace string, projectSync2PCEnabled bool) *LeaderOps {
	return &LeaderOps{
		LeaderOps:             leaderabstract.NewLeaderOps(),
		logger:                parentLogger.GetChild("mlrun"),
		namespace:             namespace,
		projectSync2PCEnabled: projectSync2PCEnabled,
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

func (l *LeaderOps) GenerateCreateProjectRequestURL(apiAddress string) string {
	return l.ProjectRequestURL(apiAddress, leaderCommon.APIVersionV1, "")
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

func (l *LeaderOps) GenerateUpdateProjectRequestURL(apiAddress, projectName string) string {
	return l.ProjectRequestURL(apiAddress, leaderCommon.APIVersionV1, projectName)
}

func (l *LeaderOps) GetDeleteExpectedStatusCode() int {
	return http.StatusNoContent
}

func (l *LeaderOps) GetDeleteStrategyHeaderName() string {
	return "x-mlrun-deletion-strategy"
}

func (l *LeaderOps) GenerateGetProjectsRequestURL(apiAddress, projectName string) string {
	return l.ProjectRequestURL(apiAddress, leaderCommon.APIVersionV1, projectName)
}

func (l *LeaderOps) GenerateGetUpdatedAfterRequestURL(apiAddress string) string {
	// TODO - for now there is no filter addition to the URL, should be added when MLRun supports updated_at
	return l.ProjectRequestURL(apiAddress, leaderCommon.APIVersionV1, "")
}

func (l *LeaderOps) GenerateDeleteProjectRequestURL(apiAddress, projectName string) string {
	return l.ProjectRequestURL(apiAddress, leaderCommon.APIVersionV2, projectName)
}

// EvaluateLeaderRequest is the entry point for every project write that originates from MLRun.
// It decides whether Nuclio should actually apply the incoming change and, if so, whether the
// operation is valid given the current state of the Kubernetes CRD.
//
// When ProjectSync2PCEnabled is false (the default) the function immediately returns
// (true, nil) — meaning "go ahead and apply". This preserves backwards compatibility
// with MLRun deployments that do not send 2PC labels at all.
//
// When 2PC is enabled the function detects which phase of the two-phase commit protocol
// the request belongs to, purely by reading the labels that MLRun attaches:
//
//   - sync-status=creating                         → Provision  (start of creation)
//   - sync-status=online,   no current-op-id       → Commit     (creation succeeded)
//   - sync-status=deleting, current-op-id present  → Mark-delete (start of deletion)
//   - sync-status=online,   current-op-id present  → Spec-update (in-place change)
//   - no sync-status (only op_id)                  → Final-delete (deletion complete)
//
// Return values:
//
//	(true,  nil)  → validation passed; the caller should apply the change to the CRD.
//	(false, nil)  → idempotent replay; the change was already applied; caller skips the write.
//	(false, err)  → validation failed; the caller should return the error to MLRun.
func (l *LeaderOps) EvaluateLeaderRequest(_ context.Context, labels map[string]string, existing platform.Project) (bool, error) {
	// Feature flag: when 2PC is disabled, every leader request is applied unconditionally.
	// This allows rolling out the 2PC protocol without requiring a simultaneous MLRun upgrade.
	if !l.projectSync2PCEnabled {
		return true, nil
	}

	// Determine the 2PC phase from the labels MLRun put on this request.
	syncStatus := labels[leaderCommon.MLRunLabelKeySyncStatus]
	currentOpID := labels[leaderCommon.MLRunLabelKeyCurrentOpID]

	switch {
	case syncStatus == leaderCommon.MLRunSyncStatusCreating:
		// Phase 0 of creation: MLRun wants to begin creating a project.
		return l.validateProvision(labels, existing)
	case syncStatus == leaderCommon.MLRunSyncStatusOnline && currentOpID == "":
		// Phase 1 of creation: MLRun says the project is ready; flip status to "online".
		return l.validateCommit(labels, existing)
	case syncStatus == leaderCommon.MLRunSyncStatusDeleting && currentOpID != "":
		// Phase 0 of deletion: MLRun wants to begin deleting a project; flip status to "deleting".
		return l.validateMarkDelete(labels, existing)
	case syncStatus == leaderCommon.MLRunSyncStatusOnline && currentOpID != "":
		// In-place update: project stays "online" but its spec changes.
		return l.validateSpecUpdate(labels, existing)
	case syncStatus == "" && currentOpID == "":
		// Phase 1 of deletion: MLRun has finished its cleanup; permanently delete the CRD.
		// Final-delete carries only op_id; current-op-id must be absent so we don't
		// silently misclassify a malformed request that forgot to set sync-status.
		return l.validateFinalDelete(labels, existing)
	default:
		// Unknown label combination — reject with a clear diagnostic.
		return false, nuclio.GetByStatusCode(http.StatusBadRequest)(
			fmt.Sprintf("Unrecognised 2PC labels: sync-status=%q current-op-id=%q",
				syncStatus, currentOpID))
	}
}

// ProjectSync2PCEnabled exposes the 2PC feature flag so callers can decide whether to
// fetch the existing CRD before invoking EvaluateLeaderRequest. When the flag is off,
// evaluation is an unconditional pass-through, so the Get would be wasted.
func (l *LeaderOps) ProjectSync2PCEnabled() bool {
	return l.projectSync2PCEnabled
}

// validateProvision validates the Provision step (sync-status=creating in request labels).
// MLRun calls this first when creating a new project. Nuclio should write the CRD
// with status=creating so that both sides know creation is in progress.
//
// Returns (true, nil)  – no CRD yet, or a stale "creating" CRD is being recovered; caller should write.
// Returns (false, nil) – same op_id already stored, idempotent replay.
func (l *LeaderOps) validateProvision(labels map[string]string, existing platform.Project) (bool, error) {
	// Every provision must carry an op_id that uniquely identifies this operation.
	requestedOpID, err := l.requireLabel(labels, leaderCommon.MLRunLabelKeyOpID, "provision")
	if err != nil {
		return false, err
	}

	// Happy path: no CRD exists yet — this is the first time we see this project.
	// Tell the caller to go ahead and create it.
	if existing == nil {
		return true, nil
	}

	// A CRD already exists; read what is currently stored on it.
	storedOpID, effectiveStatus := l.extractCRDState(existing)

	// Idempotency: the exact same provision request arrived twice (e.g. MLRun retried after a
	// timeout). The CRD is already in the correct state — skip the write and return success.
	if storedOpID == requestedOpID {
		return false, nil
	}

	// Replay protection: the incoming op_id is older than what is already stored.
	// This means an out-of-order or stale request arrived. Reject it.
	if !l.isOpIDOrdered(requestedOpID, storedOpID) {
		return false, nuclio.GetByStatusCode(http.StatusConflict)(
			fmt.Sprintf("Provision rejected: op_id %q is older than stored op_id %q (replay protection)",
				requestedOpID, storedOpID))
	}

	// The incoming op_id is newer. If the CRD is still in "creating" state this is a
	// recovery scenario: MLRun abandoned the previous provision (e.g. it crashed) and
	// is starting fresh with a new op_id. It will never send a Commit for the old op_id,
	// so there is no orphan risk. Allow the overwrite so the project is not stuck.
	if effectiveStatus == leaderCommon.MLRunSyncStatusCreating {
		return true, nil
	}

	// Reaching here means the op_id is newer but the CRD is already "online" or "deleting" —
	// the project is established and cannot be re-provisioned. Use SpecUpdate or MarkDelete.
	// (The "creating" case was handled and returned early above.)
	return false, nuclio.GetByStatusCode(http.StatusConflict)(
		fmt.Sprintf("Provision rejected: project already exists in %q state (requested op_id %q)",
			effectiveStatus, requestedOpID))
}

// validateCommit validates the Commit step: creating → online.
// MLRun calls this after the project has been fully initialised on its side.
// Nuclio should flip the CRD status from "creating" to "online".
//
// Returns (true, nil) on success; caller should update the CRD.
func (l *LeaderOps) validateCommit(labels map[string]string, existing platform.Project) (bool, error) {
	requestedOpID, err := l.requireLabel(labels, leaderCommon.MLRunLabelKeyOpID, "commit")
	if err != nil {
		return false, err
	}

	// Commit cannot happen if there is no CRD — Provision must have succeeded first.
	if err := l.requireExistingProject(existing, "Commit"); err != nil {
		return false, err
	}

	// Read the current state of the CRD.
	storedOpID, effectiveStatus := l.extractCRDState(existing)

	// Idempotency: the CRD is already online with the same op_id, meaning Nuclio already applied
	// this commit (perhaps the response timed out and MLRun is retrying). Return success without
	// writing again. This check must come before requireSyncStatus because after a successful
	// commit the status is "online", not "creating".
	if effectiveStatus == leaderCommon.MLRunSyncStatusOnline && storedOpID == requestedOpID {
		return false, nil
	}

	// The CRD must be in "creating" state — Provision must have run before Commit.
	if err := l.requireSyncStatus(effectiveStatus, leaderCommon.MLRunSyncStatusCreating, "Commit"); err != nil {
		return false, err
	}

	// The op_id on the CRD must match the one in the request to confirm we are committing
	// exactly the operation that was provisioned, not a stale or mismatched one.
	if err := l.requireOpIDMatch(requestedOpID, storedOpID, "Commit"); err != nil {
		return false, err
	}
	return true, nil
}

// validateMarkDelete validates the Mark-delete step: online → deleting.
// MLRun calls this to signal that a project is about to be deleted. Nuclio should flip
// the CRD status to "deleting" and record the new op_id so that the subsequent
// FinalDelete step can confirm it is operating on the same deletion operation.
//
// The current-op-id label acts as a CAS (compare-and-swap) key: it must match the op_id
// currently stored on the CRD so that concurrent or out-of-order requests are rejected.
// op_id is required: it becomes the new stored op_id on the CRD.
//
// Returns (true, nil) on success; caller should update the CRD.
func (l *LeaderOps) validateMarkDelete(labels map[string]string, existing platform.Project) (bool, error) {
	// The new op_id is required — it will be written to the CRD and must be present
	// so that the FinalDelete step can verify it is completing the right operation.
	newOpID, err := l.requireLabel(labels, leaderCommon.MLRunLabelKeyOpID, "mark-delete")
	if err != nil {
		return false, err
	}

	// Idempotency: the CRD is already gone. Mark-delete is phase 0 of deletion — its
	// goal is "start removing this project from Nuclio". If there is no CRD that goal
	// is already achieved and FinalDelete will hit the same idempotent branch, so
	// let the 2PC flow complete instead of forcing a reconcile with 412.
	if existing == nil {
		return false, nil
	}

	// Read the current state of the CRD.
	storedOpID, effectiveStatus := l.extractCRDState(existing)

	// Idempotency: the CRD is already in "deleting" state with the new op_id already stored,
	// meaning Nuclio already applied this mark (perhaps the response timed out). Return success
	// without writing again. This check must come before the conflict check below because the
	// same status + op_id is a replay, not a conflict.
	if effectiveStatus == leaderCommon.MLRunSyncStatusDeleting && storedOpID == newOpID {
		return false, nil
	}

	// Conflict: the CRD is already in "deleting" state with a different op_id — a
	// different delete operation is in progress and this request must not silently
	// overwrite it. Surface 409 with an explicit message rather than the generic 412
	// from requireSyncStatus below, which would say "expected online, got deleting"
	// and lose the context that the real failure is a conflicting concurrent delete.
	if effectiveStatus == leaderCommon.MLRunSyncStatusDeleting {
		return false, nuclio.GetByStatusCode(http.StatusConflict)(
			fmt.Sprintf("Different delete already in progress (requested op_id %q, stored op_id %q) [Mark-delete]",
				newOpID, storedOpID))
	}

	// The CRD must be in "online" state before it can be marked for deletion.
	if err := l.requireSyncStatus(effectiveStatus, leaderCommon.MLRunSyncStatusOnline, "Mark-delete"); err != nil {
		return false, err
	}

	// CAS check: the current-op-id in the request must equal the op_id stored on the CRD.
	// This ensures the caller is operating on the exact version it last read, preventing
	// a concurrent update from being silently overwritten. The CAS is skipped on legacy
	// CRDs that have no stored op_id yet — see requireCASOpIDMatch for the rationale.
	if err := l.requireCASOpIDMatch(labels[leaderCommon.MLRunLabelKeyCurrentOpID], storedOpID, "Mark-delete"); err != nil {
		return false, err
	}

	// The new op_id must be strictly newer than the stored one, preventing replayed or
	// out-of-order requests from overwriting a more recent operation.
	if err := l.requireNewerOpID(newOpID, storedOpID, "Mark-delete"); err != nil {
		return false, err
	}
	return true, nil
}

// validateSpecUpdate validates an in-place spec update: advances op_id while staying online.
// MLRun calls this when a project's configuration changes (e.g. labels, description).
// The status stays "online" — only the op_id advances to record that the update happened.
//
// Two ordering guards must hold:
//  1. CAS: current-op-id in the request must equal the op_id stored on the CRD.
//     This rejects updates that were computed against a stale view of the project,
//     preventing a concurrent write from being silently overwritten.
//  2. Replay protection: the new op_id must be strictly newer than the stored one
//     (UUIDv7 lexicographic order = chronological order).
//
// op_id is required: it must be written to the CRD so subsequent operations can match against it.
//
// Returns (true, nil) on success; caller should update the CRD.
func (l *LeaderOps) validateSpecUpdate(labels map[string]string, existing platform.Project) (bool, error) {
	// The new op_id is required — it will replace the stored op_id on the CRD so that
	// future operations (e.g. a subsequent update or mark-delete) can use it as a CAS key.
	newOpID, err := l.requireLabel(labels, leaderCommon.MLRunLabelKeyOpID, "spec-update")
	if err != nil {
		return false, err
	}

	// SpecUpdate cannot happen if there is no CRD — the project must exist.
	if err := l.requireExistingProject(existing, "Update"); err != nil {
		return false, err
	}

	// Read the current state of the CRD.
	storedOpID, effectiveStatus := l.extractCRDState(existing)

	// The project must be in "online" state to accept a spec update.
	// A project in "creating" or "deleting" state is not ready for user-driven changes.
	if err := l.requireSyncStatus(effectiveStatus, leaderCommon.MLRunSyncStatusOnline, "Update"); err != nil {
		return false, err
	}

	// Idempotency: the CRD is already online and the new op_id is already stored, meaning
	// Nuclio already applied this update (perhaps the response timed out). Both the status
	// check above and this op_id check together confirm the post-apply state is fully reached.
	// Return success without writing again. This must come before the CAS check because after
	// a successful update the stored op_id has advanced past the request's current-op-id.
	if storedOpID == newOpID {
		return false, nil
	}

	// CAS check: the current-op-id in the request must equal the op_id stored on the CRD.
	// This ensures the caller is operating on the exact version it last read, preventing
	// a concurrent update from being silently overwritten. The CAS is skipped on legacy
	// CRDs that have no stored op_id yet — see requireCASOpIDMatch for the rationale.
	if err := l.requireCASOpIDMatch(labels[leaderCommon.MLRunLabelKeyCurrentOpID], storedOpID, "Update"); err != nil {
		return false, err
	}

	// The new op_id must be strictly newer than the stored one, preventing replayed or
	// out-of-order requests from overwriting a more recent operation.
	if err := l.requireNewerOpID(newOpID, storedOpID, "Update"); err != nil {
		return false, err
	}
	return true, nil
}

// validateFinalDelete validates the Final-delete step.
// MLRun calls this after all internal cleanup has completed on its side.
// Nuclio should permanently remove the CRD from Kubernetes.
//
// The op_id in the request must match the one stored during Mark-delete, confirming
// that this FinalDelete is completing the exact deletion operation that was marked.
//
// Returns (false, nil) when the CRD is already gone (idempotent).
// Returns (true, nil)  on success; caller should delete the CRD.
func (l *LeaderOps) validateFinalDelete(labels map[string]string, existing platform.Project) (bool, error) {
	// The op_id must match the one written during Mark-delete — this is the handshake
	// that binds the two steps together and ensures we are deleting the right project.
	requestedOpID, err := l.requireLabel(labels, leaderCommon.MLRunLabelKeyOpID, "final-delete")
	if err != nil {
		return false, err
	}

	// Idempotency: the CRD is already gone. Nuclio previously deleted it and MLRun is
	// retrying (e.g. after a timeout). Treat this as a success.
	if existing == nil {
		return false, nil
	}

	// Read the current state of the CRD.
	storedOpID, effectiveStatus := l.extractCRDState(existing)

	// The CRD must be in "deleting" state — Mark-delete must have run before FinalDelete.
	if err := l.requireSyncStatus(effectiveStatus, leaderCommon.MLRunSyncStatusDeleting, "Final-delete"); err != nil {
		return false, err
	}

	// The op_id on the CRD must match the one in the request to confirm this FinalDelete
	// is completing the deletion that was started by the matching Mark-delete, and not
	// a stale request for a different operation.
	if err := l.requireOpIDMatch(requestedOpID, storedOpID, "Final-delete"); err != nil {
		return false, err
	}
	return true, nil
}

// requireLabel returns the value of key from labels, or a 400 Bad Request error when the
// label is absent or empty. Every 2PC phase must carry its required labels; a missing label
// means the request is malformed and MLRun should not retry it as-is.
func (l *LeaderOps) requireLabel(labels map[string]string, key, operation string) (string, error) {
	value := labels[key]
	if value == "" {
		return "", nuclio.GetByStatusCode(http.StatusBadRequest)(
			fmt.Sprintf("Missing %q label for 2PC %s", key, operation))
	}
	return value, nil
}

// requireExistingProject returns a 412 Precondition Failed when the Kubernetes CRD is nil.
// Used by phases that must operate on an existing project (Commit, MarkDelete, SpecUpdate,
// FinalDelete). Provision and FinalDelete handle the nil case themselves because for them
// a missing CRD is either expected (Provision) or idempotent (FinalDelete).
func (l *LeaderOps) requireExistingProject(existing platform.Project, operation string) error {
	if existing == nil {
		return nuclio.GetByStatusCode(http.StatusPreconditionFailed)(
			fmt.Sprintf("project CRD not found [%s]", operation))
	}
	return nil
}

// extractCRDState reads two pieces of information from the existing CRD that are needed
// by every validation: the op_id currently stored on the CRD (used for CAS checks and
// idempotency comparisons) and the effective sync-status (used to enforce phase ordering).
func (l *LeaderOps) extractCRDState(existing platform.Project) (storedOpID, syncStatus string) {
	labels := existing.GetConfig().Meta.Labels
	return labels[leaderCommon.MLRunLabelKeyOpID], l.resolveSyncStatus(labels)
}

// requireSyncStatus returns a 412 Precondition Failed when the CRD is not in the expected
// phase. This enforces the phase ordering of the 2PC protocol — for example, Commit must
// only run after Provision, meaning the CRD must already be in "creating" state.
func (l *LeaderOps) requireSyncStatus(effectiveStatus, expectedStatus, operation string) error {
	if effectiveStatus != expectedStatus {
		return nuclio.GetByStatusCode(http.StatusPreconditionFailed)(
			fmt.Sprintf("project is in %q state, expected %q [%s]",
				effectiveStatus, expectedStatus, operation))
	}
	return nil
}

// requireOpIDMatch returns a 409 Conflict when the two op_id values do not match.
// Used by phase-binding callers (Commit, FinalDelete) where the request's op_id must
// equal the one already written on the CRD — by the matching Provision or Mark-delete.
// For these callers an empty storedOpID is unreachable in practice (the preceding
// requireSyncStatus check already excludes legacy / pre-2PC CRDs), so this helper does
// not accommodate that case; use requireCASOpIDMatch for CAS-style mutations instead.
func (l *LeaderOps) requireOpIDMatch(requestedOpID, storedOpID, operation string) error {
	if storedOpID != requestedOpID {
		return nuclio.GetByStatusCode(http.StatusConflict)(
			fmt.Sprintf("op_id mismatch (requested %q, stored %q) [%s]",
				requestedOpID, storedOpID, operation))
	}
	return nil
}

// requireCASOpIDMatch is a Compare-And-Swap variant of requireOpIDMatch for the CAS-style
// mutation phases (Mark-delete, Spec-update). It treats an empty storedOpID as
// "no CAS key has been written yet" and accepts the request unconditionally, which is the
// one-shot migration path for legacy CRDs that pre-date 2PC: their op_id label is unset
// because nothing has stamped it. The current write is what stamps it, so there is nothing
// to CAS against and any non-empty current-op-id the leader sent is necessarily based on a
// stale read — rejecting it would permanently block the CRD from ever being mutated again.
// After the first leader-driven write the op_id is on the CRD and normal CAS enforcement
// resumes for every subsequent operation.
func (l *LeaderOps) requireCASOpIDMatch(requestedOpID, storedOpID, operation string) error {
	if storedOpID == "" {
		return nil
	}
	return l.requireOpIDMatch(requestedOpID, storedOpID, operation)
}

// requireNewerOpID returns a 409 Conflict when the incoming op_id is not strictly newer
// than the one already stored on the CRD. This is the replay-protection guard: because
// UUIDv7 embeds a millisecond timestamp in its most-significant bits, a lexicographically
// smaller value means the request is older and must be rejected to prevent stale writes.
func (l *LeaderOps) requireNewerOpID(newOpID, storedOpID, operation string) error {
	if newOpID != "" && !l.isOpIDOrdered(newOpID, storedOpID) {
		return nuclio.GetByStatusCode(http.StatusConflict)(
			fmt.Sprintf("new op_id %q is not newer than stored op_id %q, replay protection [%s]",
				newOpID, storedOpID, operation))
	}
	return nil
}

// resolveSyncStatus returns the sync-status label value from the CRD's labels.
// If the label is absent the CRD pre-dates the 2PC introduction, so we treat it as
// "online" — the last known good state before 2PC was added.
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
