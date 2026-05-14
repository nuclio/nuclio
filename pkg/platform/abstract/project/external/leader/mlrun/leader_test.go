//go:build test_unit

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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/nuclio/nuclio/pkg/platform"
	leaderCommon "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type LeaderTestSuite struct {
	suite.Suite
	logger    logger.Logger
	leaderOps *LeaderOps
	namespace string
}

func (suite *LeaderTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test-mlrun-leader")
	suite.Require().NoError(err)
	suite.namespace = "test-namespace"
	suite.leaderOps = NewLeaderOps(suite.logger, suite.namespace, true)
}

func (suite *LeaderTestSuite) TestGenerateProjectRequestBody() {
	tests := []struct {
		name        string
		project     *platform.ProjectConfig
		expectError bool
	}{
		{
			name: "ValidProject",
			project: &platform.ProjectConfig{
				Meta: platform.ProjectMeta{Name: "test"},
			},
		},
		{
			name:        "NilProject",
			project:     nil,
			expectError: true,
		},
	}

	for _, testCase := range tests {
		suite.Run(testCase.name, func() {
			result, err := suite.leaderOps.GenerateProjectRequestBody(testCase.project)
			if testCase.expectError {
				suite.Require().Error(err)
				suite.Require().Nil(result)
			} else {
				suite.Require().NoError(err)
				suite.Require().NotNil(result)
				var resultProject MLRunProject
				suite.Require().NoError(json.Unmarshal(result, &resultProject))
				suite.Require().Equal(testCase.project.Meta.Name, resultProject.Metadata.Name)
			}
		})
	}
}

func (suite *LeaderTestSuite) TestGenerateProjectDeletionRequestBody() {
	suite.Run("ValidProjectName", func() {
		result, err := suite.leaderOps.GenerateProjectDeletionRequestBody("my-project")
		suite.Require().NoError(err)
		var project MLRunProject
		suite.Require().NoError(json.Unmarshal(result, &project))
		suite.Require().Equal("my-project", project.Metadata.Name)
	})
}

func (suite *LeaderTestSuite) TestResolveCreateProjectResponse() {
	testCases := []struct {
		name        string
		body        []byte
		expectError bool
	}{
		{
			name: "ValidResponse",
			body: func() []byte {
				b, _ := json.Marshal(MLRunProject{Metadata: ProjectMetadata{Name: "test-project"}})
				return b
			}(),
		},
		{
			name:        "InvalidResponse",
			body:        []byte(`not-json`),
			expectError: true,
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			resp, err := suite.leaderOps.ResolveCreateProjectResponse(context.TODO(), testCase.body)
			if testCase.expectError {
				suite.Require().Error(err)
				suite.Require().Nil(resp)
			} else {
				suite.Require().NoError(err)
				suite.Require().NotNil(resp)
				suite.Require().Equal(resp.GetLastJobID(), "")
				responseProject, ok := resp.(*MLRunProject)
				suite.Require().True(ok)
				suite.Require().Equal("test-project", responseProject.Metadata.Name)
			}
		})
	}
}

func (suite *LeaderTestSuite) TestResolveGetProjectResponse() {
	testCases := []struct {
		name        string
		body        []byte
		expectedLen int
	}{
		{
			name:        "multipleProjects",
			body:        []byte(`{"projects":[{"kind":"project","metadata":{"name":"curl-test","created":"2025-08-18T14:51:52.330000","labels":{},"annotations":{}},"spec":{"description":null,"owner":"admin","goals":null,"params":{},"functions":[],"workflows":[],"artifacts":[],"artifact_path":null,"conda":null,"source":null,"subpath":null,"origin_url":null,"desired_state":"online","custom_packagers":null,"default_image":null,"build":null},"status":{"state":"online"}},{"kind":"project","metadata":{"name":"curl-test2","created":"2025-08-20T06:25:30.227000","labels":{},"annotations":{}},"spec":{"description":null,"owner":"admin","goals":null,"params":{},"functions":[],"workflows":[],"artifacts":[],"artifact_path":null,"conda":null,"source":null,"subpath":null,"origin_url":null,"desired_state":"online","custom_packagers":null,"default_image":null,"build":null},"status":{"state":"online"}},{"kind":"project","metadata":{"name":"default","created":"2025-05-12T11:18:51.038000","labels":{},"annotations":{}},"spec":{"description":"Default Project","owner":null,"goals":null,"params":{},"functions":[],"workflows":[],"artifacts":[],"artifact_path":null,"conda":null,"source":null,"subpath":null,"origin_url":null,"desired_state":"online","custom_packagers":null,"default_image":null,"build":null},"status":{"state":"online"}}]}`),
			expectedLen: 3,
		},
		{
			name:        "noProjects",
			body:        []byte(`{"projects":[]}`),
			expectedLen: 0,
		},
		{
			name:        "singleProject",
			body:        []byte(`{"projects":[{"kind":"project","metadata":{"name":"asd","created":"2025-09-03T06:55:30.824045","labels":{},"annotations":{},"namespace":"mlrun"},"spec":{"description":null,"owner":null,"goals":null,"params":{},"functions":[],"workflows":[],"artifacts":[],"artifact_path":null,"conda":null,"source":null,"subpath":null,"origin_url":null,"desired_state":"online","custom_packagers":null,"default_image":null,"build":null,"default_function_node_selector":{}},"status":{"state":"online"}}]}`),
			expectedLen: 1,
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			projects, err := suite.leaderOps.ResolveGetProjectResponse(false, testCase.body)
			suite.Require().NoError(err)
			suite.Require().Len(projects, testCase.expectedLen)
			for _, project := range projects {
				suite.Require().Equal(suite.namespace, project.GetConfig().Meta.Namespace)
			}
		})
	}
}

func (suite *LeaderTestSuite) TestParseJobStatusResponse() {
	resp, ok := suite.leaderOps.ParseJobStatusResponse(context.TODO(), nil)
	suite.Require().Nil(resp)
	suite.Require().False(ok)
}

func (suite *LeaderTestSuite) TestGenerateCreateProjectRequestURL() {
	testCases := []struct {
		name     string
		address  string
		expected string
	}{
		{
			name:     "Basic",
			address:  "http://localhost/api",
			expected: "http://localhost/api/v1/projects",
		},
		{
			name:     "WithoutHttpPrefix",
			address:  "some-address",
			expected: "some-address/v1/projects",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			url := suite.leaderOps.GenerateCreateProjectRequestURL(testCase.address)
			suite.Require().Equal(testCase.expected, url)
		})
	}
}

func (suite *LeaderTestSuite) TestHandleCreateResponseErr() {
	testCases := []struct {
		name         string
		body         []byte
		response     *http.Response
		expectErrStr string
	}{
		{
			name: "MLRunError",
			body: func() []byte {
				b, _ := json.Marshal(MlrunError{Detail: "some error"})
				return b
			}(),
			response:     &http.Response{StatusCode: http.StatusBadRequest},
			expectErrStr: "some error",
		},
		{
			name:         "NonMLRunError",
			body:         []byte(`not-json`),
			response:     &http.Response{StatusCode: http.StatusBadRequest},
			expectErrStr: "Failed to send request to leader",
		},
		{
			name: "NilResponse",
			body: func() []byte {
				b, _ := json.Marshal(MlrunError{Detail: "some error"})
				return b
			}(),
			response:     nil,
			expectErrStr: "Failed to get response from leader, response is nil",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			err := suite.leaderOps.HandleCreateResponseErr(context.TODO(), testCase.body, testCase.response, errors.New(""))
			suite.Require().Error(err)
			suite.Require().Equal(err.Error(), testCase.expectErrStr)
		})
	}
}

func (suite *LeaderTestSuite) TestIsJobCompleted() {
	suite.Run("AlwaysNil", func() {
		err := suite.leaderOps.IsJobCompleted(context.TODO(), nil, "")
		suite.Require().NoError(err)
	})
}

func (suite *LeaderTestSuite) TestGenerateUpdateProjectRequestURL() {
	testCases := []struct {
		name        string
		address     string
		projectName string
		expected    string
	}{
		{
			name:        "Basic",
			address:     "http://localhost/api",
			projectName: "test-project",
			expected:    "http://localhost/api/v1/projects/test-project",
		},
		{
			name:        "WithEmptyUrl",
			address:     "",
			projectName: "test-project",
			expected:    "/v1/projects/test-project",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			url := suite.leaderOps.GenerateUpdateProjectRequestURL(testCase.address, testCase.projectName)
			suite.Require().Equal(testCase.expected, url)
		})
	}
}

func (suite *LeaderTestSuite) TestGetDeleteExpectedStatusCode() {
	suite.Run("AlwaysNoContent", func() {
		code := suite.leaderOps.GetDeleteExpectedStatusCode()
		suite.Require().Equal(http.StatusNoContent, code)
	})
}

func (suite *LeaderTestSuite) TestGetDeleteStrategyHeaderName() {
	header := suite.leaderOps.GetDeleteStrategyHeaderName()
	suite.Require().Equal(header, "x-mlrun-deletion-strategy")
}

func (suite *LeaderTestSuite) TestGenerateGetProjectsRequestURL() {
	url := suite.leaderOps.GenerateGetProjectsRequestURL("http://localhost/api", "b")
	suite.Require().Equal("http://localhost/api/v1/projects/b", url)
}

func (suite *LeaderTestSuite) TestGenerateGetUpdatedAfterRequestURL() {
	url := suite.leaderOps.GenerateGetUpdatedAfterRequestURL("test")
	suite.Require().Equal("test/projects", url)
}

func (suite *LeaderTestSuite) TestGenerateDeleteProjectRequestURL() {
	url := suite.leaderOps.GenerateDeleteProjectRequestURL("http://localhost/api", "test-project")
	suite.Require().Equal("http://localhost/api/v2/projects/test-project", url)
}

func (suite *LeaderTestSuite) TestShouldWaitForCreateCompletion() {
	suite.Require().False(suite.leaderOps.ShouldWaitForCreateCompletion())
}

func (suite *LeaderTestSuite) TestEvaluateLeaderRequest_Provision() {
	// Lexicographic ordering: opOld < opStored < opNew
	const opOld = "018d-op"
	const opStored = "018e-op"
	const opNew = "018f-op"

	testCases := []struct {
		name            string
		requestLabels   map[string]string
		existingProject platform.Project
		wantApply       bool
		wantStatusCode  int
	}{
		{
			name:            "NoCRDShouldCreate",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusCreating, opStored, ""),
			existingProject: nil,
			wantApply:       true,
		},
		{
			name:            "SameOpIDIdempotent",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusCreating, opStored, ""),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusCreating, opStored, ""),
			wantApply:       false,
		},
		{
			name:            "OlderOpIDReplayProtection",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusCreating, opOld, ""),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusCreating, opStored, ""),
			wantApply:       false,
			wantStatusCode:  http.StatusConflict,
		},
		{
			name:            "NewerOpIDProjectAlreadyOnline",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusCreating, opNew, ""),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusOnline, opStored, ""),
			wantApply:       false,
			// project exists and is live; a new Provision cannot overwrite it
			wantStatusCode: http.StatusConflict,
		},
		{
			name:            "NewerOpIDRecoverStaleCRD",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusCreating, opNew, ""),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusCreating, opStored, ""),
			wantApply:       true,
			// MLRun abandoned the old provision and is starting fresh; allow the overwrite
		},
		{
			name:            "OnlineSameOpIDIdempotent",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusCreating, opStored, ""),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusOnline, opStored, ""),
			wantApply:       false,
			// op_id already stored — idempotency guard fires before the status check
		},
		{
			name:            "OnlineOlderOpIDReplayProtection",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusCreating, opOld, ""),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusOnline, opStored, ""),
			wantApply:       false,
			wantStatusCode:  http.StatusConflict,
		},
		{
			name:            "DeletingSameOpIDIdempotent",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusCreating, opStored, ""),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusDeleting, opStored, ""),
			wantApply:       false,
			// op_id already stored — idempotency guard fires before the status check
		},
		{
			name:            "DeletingOlderOpIDReplayProtection",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusCreating, opOld, ""),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusDeleting, opStored, ""),
			wantApply:       false,
			wantStatusCode:  http.StatusConflict,
		},
		{
			name:            "DeletingNewerOpIDAlreadyExists",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusCreating, opNew, ""),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusDeleting, opStored, ""),
			wantApply:       false,
			// project is in deleting state; cannot re-provision
			wantStatusCode: http.StatusConflict,
		},
		{
			name:           "MissingOpIDLabel",
			requestLabels:  requestLabels(leaderCommon.MLRunSyncStatusCreating, "", ""),
			wantApply:      false,
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			shouldApply, err := suite.leaderOps.EvaluateLeaderRequest(context.TODO(), testCase.requestLabels, testCase.existingProject)
			suite.Require().Equal(testCase.wantApply, shouldApply)
			if testCase.wantStatusCode != 0 {
				suite.Require().Error(err)
				suite.Require().Equal(testCase.wantStatusCode, statusCodeOf(err))
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *LeaderTestSuite) TestEvaluateLeaderRequest_Commit() {
	const opStored = "018e-op"
	const opOther = "018f-op"

	testCases := []struct {
		name            string
		requestLabels   map[string]string
		existingProject platform.Project
		wantApply       bool
		wantStatusCode  int
	}{
		{
			name:            "ValidCommit",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusOnline, opStored, ""),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusCreating, opStored, ""),
			wantApply:       true,
		},
		{
			name:           "NoCRD",
			requestLabels:  requestLabels(leaderCommon.MLRunSyncStatusOnline, opStored, ""),
			wantApply:      false,
			wantStatusCode: http.StatusPreconditionFailed,
		},
		{
			name:            "WrongStatusDeleting",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusOnline, opStored, ""),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusDeleting, opStored, ""),
			wantApply:       false,
			wantStatusCode:  http.StatusPreconditionFailed, // deleting != creating
		},
		{
			name:            "WrongStatusOnlineDifferentOpID",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusOnline, opOther, ""),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusOnline, opStored, ""),
			wantApply:       false,
			// status is already online but op_id differs — idempotency guard does not fire,
			// then requireSyncStatus(online, creating) → 412
			wantStatusCode: http.StatusPreconditionFailed,
		},
		{
			name:            "OpIDMismatch",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusOnline, opOther, ""),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusCreating, opStored, ""),
			wantApply:       false,
			wantStatusCode:  http.StatusConflict,
		},
		{
			name:           "MissingOpIDLabel",
			requestLabels:  requestLabels(leaderCommon.MLRunSyncStatusOnline, "", ""),
			wantApply:      false,
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:            "AlreadyCommittedIdempotent",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusOnline, opStored, ""),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusOnline, opStored, ""),
			wantApply:       false,
			// no error: CRD is already online with the same op_id, commit is a no-op
		},
		{
			name:            "BackwardsCompatNoCRDLabel",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusOnline, opStored, ""),
			existingProject: stubProject("", opStored, ""), // no sync-status → defaults to online
			wantApply:       false,
			// idempotency guard fires first: status defaults to online and op_id matches
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			shouldApply, err := suite.leaderOps.EvaluateLeaderRequest(context.TODO(), testCase.requestLabels, testCase.existingProject)
			suite.Require().Equal(testCase.wantApply, shouldApply)
			if testCase.wantStatusCode != 0 {
				suite.Require().Error(err)
				suite.Require().Equal(testCase.wantStatusCode, statusCodeOf(err))
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *LeaderTestSuite) TestEvaluateLeaderRequest_MarkDelete() {
	const opOld = "018d-op"
	const opStored = "018e-op"
	const opNew = "018f-op"

	testCases := []struct {
		name            string
		requestLabels   map[string]string
		existingProject platform.Project
		wantApply       bool
		wantStatusCode  int
	}{
		{
			name:            "ValidMarkDelete",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusDeleting, opNew, opStored),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusOnline, opStored, ""),
			wantApply:       true,
		},
		{
			name:            "NoCRDIdempotent",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusDeleting, opNew, opStored),
			existingProject: nil,
			wantApply:       false,
			// no error: mark-delete's goal is to start removing the project; if the
			// CRD is already gone the goal is achieved and FinalDelete will also skip,
			// so let the 2PC flow complete instead of forcing a reconcile.
		},
		{
			name:            "WrongStatus",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusDeleting, opNew, opStored),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusCreating, opStored, ""),
			wantApply:       false,
			wantStatusCode:  http.StatusPreconditionFailed,
		},
		{
			name:            "CASKeyMismatch",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusDeleting, opNew, opOld),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusOnline, opStored, ""),
			wantApply:       false,
			wantStatusCode:  http.StatusConflict,
		},
		{
			name:            "NewOpIDOlderThanStored",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusDeleting, opOld, opStored),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusOnline, opStored, ""),
			wantApply:       false,
			wantStatusCode:  http.StatusConflict,
		},
		{
			name:           "MissingOpIDLabel",
			requestLabels:  requestLabels(leaderCommon.MLRunSyncStatusDeleting, "", opStored),
			wantApply:      false,
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:            "AlreadyMarkDeletedIdempotent",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusDeleting, opNew, opStored),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusDeleting, opNew, ""),
			wantApply:       false,
			// no error: CRD is already deleting with opNew stored, mark-delete is a no-op
		},
		{
			name:            "AlreadyDeletingDifferentOpIDConflict",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusDeleting, opNew, opStored),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusDeleting, opOld, ""),
			wantApply:       false,
			// CRD is already being deleted by a different operation — 409 Conflict
			// with an explicit diagnostic, not the generic 412 that the online-state
			// precondition would produce.
			wantStatusCode: http.StatusConflict,
		},
		{
			// Legacy CRD created before 2PC was enabled: it has no op_id label and
			// resolveSyncStatus defaults its status to "online". CAS must be skipped (there
			// is nothing to swap against) so the first leader-driven mark-delete can stamp
			// op_id onto the CRD. Subsequent operations go through the normal CAS path.
			name:            "LegacyCRDNoStoredOpIDBootstraps",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusDeleting, opNew, opStored),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusOnline, "", ""),
			wantApply:       true,
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			shouldApply, err := suite.leaderOps.EvaluateLeaderRequest(context.TODO(), testCase.requestLabels, testCase.existingProject)
			suite.Require().Equal(testCase.wantApply, shouldApply)
			if testCase.wantStatusCode != 0 {
				suite.Require().Error(err)
				suite.Require().Equal(testCase.wantStatusCode, statusCodeOf(err))
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *LeaderTestSuite) TestEvaluateLeaderRequest_SpecUpdate() {
	const opOld = "018d-op"
	const opStored = "018e-op"
	const opNew = "018f-op"

	testCases := []struct {
		name            string
		requestLabels   map[string]string
		existingProject platform.Project
		wantApply       bool
		wantStatusCode  int
	}{
		{
			name:            "ValidSpecUpdate",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusOnline, opNew, opStored),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusOnline, opStored, ""),
			wantApply:       true,
		},
		{
			name:            "NoCRD",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusOnline, opNew, opStored),
			existingProject: nil,
			wantApply:       false,
			wantStatusCode:  http.StatusPreconditionFailed,
		},
		{
			name:            "WrongStatusCreating",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusOnline, opNew, opStored),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusCreating, opStored, ""),
			wantApply:       false,
			wantStatusCode:  http.StatusPreconditionFailed,
		},
		{
			name:            "WrongStatusDeleting",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusOnline, opNew, opStored),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusDeleting, opStored, ""),
			wantApply:       false,
			wantStatusCode:  http.StatusPreconditionFailed,
		},
		{
			name:          "StaleCASKeyRejected",
			requestLabels: requestLabels(leaderCommon.MLRunSyncStatusOnline, opNew, opOld),
			// current-op-id (opOld) does not match stored (opStored): the caller
			// computed this update against a stale view of the CRD, so reject with 409
			// rather than silently overwriting a concurrent update.
			existingProject: stubProject(leaderCommon.MLRunSyncStatusOnline, opStored, ""),
			wantApply:       false,
			wantStatusCode:  http.StatusConflict,
		},
		{
			name:            "NewOpIDOlderThanStored",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusOnline, opOld, opStored),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusOnline, opStored, ""),
			wantApply:       false,
			wantStatusCode:  http.StatusConflict,
		},
		{
			name:            "AlreadyAppliedIdempotent",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusOnline, opNew, opStored),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusOnline, opNew, ""),
			wantApply:       false,
			// no error: CRD is already online with opNew stored, spec update is a no-op
		},
		{
			name:           "MissingOpIDLabel",
			requestLabels:  requestLabels(leaderCommon.MLRunSyncStatusOnline, "", opStored),
			wantApply:      false,
			wantStatusCode: http.StatusBadRequest,
		},
		{
			// Legacy CRD created before 2PC was enabled: it has no op_id label and
			// resolveSyncStatus defaults its status to "online". CAS must be skipped so
			// the first leader-driven spec-update can stamp op_id onto the CRD; without
			// this bootstrap the CRD would be permanently un-updatable.
			name:            "LegacyCRDNoStoredOpIDBootstraps",
			requestLabels:   requestLabels(leaderCommon.MLRunSyncStatusOnline, opNew, opStored),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusOnline, "", ""),
			wantApply:       true,
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			shouldApply, err := suite.leaderOps.EvaluateLeaderRequest(context.TODO(), testCase.requestLabels, testCase.existingProject)
			suite.Require().Equal(testCase.wantApply, shouldApply)
			if testCase.wantStatusCode != 0 {
				suite.Require().Error(err)
				suite.Require().Equal(testCase.wantStatusCode, statusCodeOf(err))
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *LeaderTestSuite) TestEvaluateLeaderRequest_FinalDelete() {
	const opStored = "018e-op"
	const opOther = "018f-op"

	testCases := []struct {
		name            string
		requestLabels   map[string]string
		existingProject platform.Project
		wantApply       bool
		wantStatusCode  int
	}{
		{
			name:            "NoCRDIdempotent",
			requestLabels:   requestLabels("", opStored, ""),
			existingProject: nil,
			wantApply:       false,
		},
		{
			name:            "ValidFinalDelete",
			requestLabels:   requestLabels("", opStored, ""),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusDeleting, opStored, ""),
			wantApply:       true,
		},
		{
			name:            "WrongStatusCreating",
			requestLabels:   requestLabels("", opStored, ""),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusCreating, opStored, ""),
			wantApply:       false,
			wantStatusCode:  http.StatusPreconditionFailed,
		},
		{
			name:            "WrongStatusOnline",
			requestLabels:   requestLabels("", opStored, ""),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusOnline, opStored, ""),
			wantApply:       false,
			wantStatusCode:  http.StatusPreconditionFailed,
		},
		{
			name:            "OpIDMismatch",
			requestLabels:   requestLabels("", opOther, ""),
			existingProject: stubProject(leaderCommon.MLRunSyncStatusDeleting, opStored, ""),
			wantApply:       false,
			wantStatusCode:  http.StatusConflict,
		},
		{
			name:           "MissingOpIDLabel",
			requestLabels:  requestLabels("", "", ""),
			wantApply:      false,
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			shouldApply, err := suite.leaderOps.EvaluateLeaderRequest(context.TODO(), testCase.requestLabels, testCase.existingProject)
			suite.Require().Equal(testCase.wantApply, shouldApply)
			if testCase.wantStatusCode != 0 {
				suite.Require().Error(err)
				suite.Require().Equal(testCase.wantStatusCode, statusCodeOf(err))
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *LeaderTestSuite) TestEvaluateLeaderRequest_InvalidLabels() {
	testCases := []struct {
		name           string
		requestLabels  map[string]string
		wantStatusCode int
	}{
		{
			name:           "DeletingWithoutCurrentOpID",
			requestLabels:  requestLabels(leaderCommon.MLRunSyncStatusDeleting, "018e-op", ""),
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "UnknownSyncStatus",
			requestLabels:  requestLabels("unknown-status", "018e-op", ""),
			wantStatusCode: http.StatusBadRequest,
		},
		{
			// final-delete requires sync-status absent AND current-op-id absent;
			// presence of current-op-id without sync-status is ambiguous, so reject.
			name:           "EmptySyncStatusWithCurrentOpID",
			requestLabels:  requestLabels("", "018e-op", "018d-op"),
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			shouldApply, err := suite.leaderOps.EvaluateLeaderRequest(context.TODO(), testCase.requestLabels, nil)
			suite.Require().False(shouldApply)
			suite.Require().Error(err)
			suite.Require().Equal(testCase.wantStatusCode, statusCodeOf(err))
		})
	}
}

// statusCodeOf extracts the HTTP status code from a nuclio ErrorWithStatusCode.
// Returns 0 when the error does not carry a status code.
func statusCodeOf(err error) int {
	type statusCoder interface{ StatusCode() int }
	var statusErr statusCoder
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode()
	}
	return 0
}

// stubProject builds a platform.AbstractProject carrying the given 2PC label
// values on its meta. Pass empty strings to omit a label.
func stubProject(syncStatus, opID, currentOpID string) *platform.AbstractProject {
	return &platform.AbstractProject{
		ProjectConfig: platform.ProjectConfig{
			Meta: platform.ProjectMeta{Labels: requestLabels(syncStatus, opID, currentOpID)},
		},
	}
}

// requestLabels builds the inbound 2PC label map for a test request.
func requestLabels(syncStatus, opID, currentOpID string) map[string]string {
	labels := map[string]string{}
	if syncStatus != "" {
		labels[leaderCommon.MLRunLabelKeySyncStatus] = syncStatus
	}
	if opID != "" {
		labels[leaderCommon.MLRunLabelKeyOpID] = opID
	}
	if currentOpID != "" {
		labels[leaderCommon.MLRunLabelKeyCurrentOpID] = currentOpID
	}
	return labels
}

func TestLeaderTestSuite(t *testing.T) {
	suite.Run(t, new(LeaderTestSuite))
}
