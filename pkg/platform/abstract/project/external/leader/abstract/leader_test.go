//go:build test_unit

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

package abstract

import (
	"context"
	"net/http"
	"testing"

	"github.com/nuclio/nuclio/pkg/auth"
	leaderCommon "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"

	"github.com/stretchr/testify/suite"
)

// stubSession is a minimal auth.Session used only to verify that
// AddAuthSessionHeaders wires the compiled authorization header through.
type stubSession struct{}

func (s *stubSession) GetUsername() string              { return "" }
func (s *stubSession) GetPassword() string              { return "" }
func (s *stubSession) GetUserID() string                { return "" }
func (s *stubSession) GetGroupIDs() []string            { return nil }
func (s *stubSession) GetUserLabels() map[string]string { return nil }
func (s *stubSession) CompileAuthorizationHeader() string {
	return "Bearer stub-token"
}

var _ auth.Session = &stubSession{}

type LeaderOpsTestSuite struct {
	suite.Suite
	leaderOps *LeaderOps
}

func (suite *LeaderOpsTestSuite) SetupSuite() {
	suite.leaderOps = NewLeaderOps()
}

func (suite *LeaderOpsTestSuite) TestShouldWaitForCreateCompletion() {
	suite.Require().False(suite.leaderOps.ShouldWaitForCreateCompletion())
}

func (suite *LeaderOpsTestSuite) TestGetJobStatusRequestCookies() {
	suite.Require().Nil(suite.leaderOps.GetJobStatusRequestCookies(nil))
}

func (suite *LeaderOpsTestSuite) TestGetJobRequestFilter() {
	suite.Require().Equal("", suite.leaderOps.GetJobRequestFilter(nil))
}

func (suite *LeaderOpsTestSuite) TestGetExpectedStatusCode() {
	for _, testCase := range []struct {
		name      string
		operation leaderCommon.ProjectOperation
		expected  int
	}{
		{name: "Create", operation: leaderCommon.ProjectOperationCreate, expected: http.StatusCreated},
		{name: "Update", operation: leaderCommon.ProjectOperationUpdate, expected: http.StatusOK},
		{name: "Delete", operation: leaderCommon.ProjectOperationDelete, expected: http.StatusAccepted},
		{name: "Unknown", operation: leaderCommon.ProjectOperation("unknown"), expected: http.StatusInternalServerError},
	} {
		suite.Run(testCase.name, func() {
			code := suite.leaderOps.GetExpectedStatusCode(testCase.operation)
			suite.Require().Equal(testCase.expected, code)
		})
	}
}

func (suite *LeaderOpsTestSuite) TestGetAuthSessionCookie() {
	suite.Require().Nil(suite.leaderOps.GetAuthSessionCookie(&stubSession{}))
}

func (suite *LeaderOpsTestSuite) TestAddAuthSessionHeaders() {
	headers := map[string]string{}
	suite.leaderOps.AddAuthSessionHeaders(headers, &stubSession{})
	suite.Require().Equal("Bearer stub-token", headers["authorization"])
}

func (suite *LeaderOpsTestSuite) TestParseJobStatusResponse() {
	resp, terminated := suite.leaderOps.ParseJobStatusResponse(context.TODO(), nil)
	suite.Require().Nil(resp)
	suite.Require().False(terminated)
}

func (suite *LeaderOpsTestSuite) TestGetJobIdUrl() {
	suite.Require().Equal("", suite.leaderOps.GetJobIdUrl("address", "job-id"))
}

func (suite *LeaderOpsTestSuite) TestIsJobCompleted() {
	suite.Require().NoError(suite.leaderOps.IsJobCompleted(context.TODO(), nil, "project"))
}

func (suite *LeaderOpsTestSuite) TestEvaluateLeaderRequest() {
	shouldApply, err := suite.leaderOps.EvaluateLeaderRequest(context.TODO(), map[string]string{}, nil)
	suite.Require().NoError(err)
	suite.Require().True(shouldApply)
}

func (suite *LeaderOpsTestSuite) TestProjectSync2PCEnabled() {
	suite.Require().False(suite.leaderOps.ProjectSync2PCEnabled())
}

func TestLeaderOpsTestSuite(t *testing.T) {
	suite.Run(t, new(LeaderOpsTestSuite))
}
