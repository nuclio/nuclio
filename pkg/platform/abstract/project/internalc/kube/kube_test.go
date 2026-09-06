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

package kube

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project"
	leaderCommon "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	nuclioclient "github.com/nuclio/nuclio/pkg/platform/kube/clients/nuclio"
	nuclioclientmock "github.com/nuclio/nuclio/pkg/platform/kube/clients/nuclio/mock"

	"github.com/nuclio/logger"
	nuclio "github.com/nuclio/nuclio-sdk-go"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	testNamespace = "test-namespace"
	testProject   = "test-project"
)

var testUpdatedAt = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

type FollowerTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *FollowerTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test-internalc-kube")
	suite.Require().NoError(err)
}

func (suite *FollowerTestSuite) TestPrepareCreate() {
	for _, testCase := range []struct {
		name          string
		existing      *nuclioio.NuclioProject
		mockWrites    func(*nuclioclientmock.Client)
		opID          string
		owner         string
		expectedState *platform.Project2PCState
		expectedError bool
		expectedCode  int
	}{
		{
			name: "HappyPathCreatesCRD",
			mockWrites: func(m *nuclioclientmock.Client) {
				m.On("CreateNuclioProject", mock.Anything, testNamespace, mock.MatchedBy(func(p *nuclioio.NuclioProject) bool {
					return p.Labels[leaderCommon.OrisLabelKeyOpID] == "op-1" &&
						p.Labels[leaderCommon.OrisLabelKeySyncStatus] == string(leaderCommon.OrisSyncStatusCreating) &&
						p.Status.UpdatedAt != nil
				})).Return(&nuclioio.NuclioProject{}, nil).Once()
			},
			opID:          "op-1",
			expectedState: &platform.Project2PCState{Name: testProject, OpID: "op-1", SyncStatus: "creating"},
		},
		{
			name:          "IdempotentReplaySameOpID",
			existing:      newProjectFixture("op-1", leaderCommon.OrisSyncStatusCreating),
			opID:          "op-1",
			expectedState: &platform.Project2PCState{Name: testProject, OpID: "op-1", SyncStatus: "creating"},
		},
		{
			name:          "OlderOpIDRejected",
			existing:      newProjectFixture("op-2", leaderCommon.OrisSyncStatusCreating),
			opID:          "op-1",
			expectedError: true,
			expectedCode:  409,
		},
		{
			name:     "RecoversAbandonedProvisionWithNewerOpID",
			existing: newProjectFixture("op-1", leaderCommon.OrisSyncStatusCreating),
			mockWrites: func(m *nuclioclientmock.Client) {
				m.On("UpdateNuclioProject", mock.Anything, testNamespace, mock.MatchedBy(func(p *nuclioio.NuclioProject) bool {
					return p.Labels[leaderCommon.OrisLabelKeyOpID] == "op-2" &&
						p.Labels[leaderCommon.OrisLabelKeySyncStatus] == string(leaderCommon.OrisSyncStatusCreating) &&
						p.Spec.Owner == "second-attempt"
				})).Return(&nuclioio.NuclioProject{}, nil).Once()
			},
			opID:          "op-2",
			owner:         "second-attempt",
			expectedState: &platform.Project2PCState{Name: testProject, OpID: "op-2", SyncStatus: "creating"},
		},
		{
			name:          "RejectedWhenAlreadyOnline",
			existing:      newProjectFixture("op-1", leaderCommon.OrisSyncStatusOnline),
			opID:          "op-2",
			expectedError: true,
			expectedCode:  409,
		},
	} {
		suite.Run(testCase.name, func() {
			mockClient := &nuclioclientmock.Client{}
			var getErr error
			if testCase.existing == nil {
				getErr = apierrors.NewNotFound(schema.GroupResource{}, testProject)
			}
			mockClient.On("GetNuclioProject", mock.Anything, testNamespace, testProject).Return(testCase.existing, getErr)
			if testCase.mockWrites != nil {
				testCase.mockWrites(mockClient)
			}

			client := suite.newClient(mockClient)
			state, err := client.PrepareCreate(context.TODO(), &platform.PrepareCreateProjectOptions{
				ProjectConfig: platform.ProjectConfig{
					Meta: platform.ProjectMeta{Name: testProject, Namespace: testNamespace},
					Spec: platform.ProjectSpec{Owner: testCase.owner},
				},
				OpID: testCase.opID,
			})

			if testCase.expectedError {
				suite.Require().Error(err)
				var statusErr nuclio.WithStatusCode
				suite.Require().True(errors.As(err, &statusErr))
				suite.Require().Equal(testCase.expectedCode, statusErr.StatusCode())
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(testCase.expectedState, state)
			}
			mockClient.AssertExpectations(suite.T())
		})
	}
}

func (suite *FollowerTestSuite) TestCommitCreate() {
	for _, testCase := range []struct {
		name          string
		existing      *nuclioio.NuclioProject
		mockWrites    func(*nuclioclientmock.Client)
		opID          string
		expectedState *platform.Project2PCState
		expectedError bool
		expectedCode  int
	}{
		{
			name:     "HappyPathFlipsToOnline",
			existing: newProjectFixture("op-1", leaderCommon.OrisSyncStatusCreating),
			mockWrites: func(m *nuclioclientmock.Client) {
				m.On("UpdateNuclioProject", mock.Anything, testNamespace, mock.MatchedBy(func(p *nuclioio.NuclioProject) bool {
					return p.Labels[leaderCommon.OrisLabelKeyOpID] == "op-1" &&
						p.Labels[leaderCommon.OrisLabelKeySyncStatus] == string(leaderCommon.OrisSyncStatusOnline)
				})).Return(&nuclioio.NuclioProject{}, nil).Once()
			},
			opID:          "op-1",
			expectedState: &platform.Project2PCState{Name: testProject, OpID: "op-1", SyncStatus: "online"},
		},
		{
			name:          "IdempotentReplay",
			existing:      newProjectFixture("op-1", leaderCommon.OrisSyncStatusOnline),
			opID:          "op-1",
			expectedState: &platform.Project2PCState{Name: testProject, OpID: "op-1", SyncStatus: "online"},
		},
		{
			name:          "RejectedWhenNoCRD",
			opID:          "op-1",
			expectedError: true,
			expectedCode:  412,
		},
		{
			name:          "RejectedOnOpIDMismatch",
			existing:      newProjectFixture("op-1", leaderCommon.OrisSyncStatusCreating),
			opID:          "op-2",
			expectedError: true,
			expectedCode:  409,
		},
	} {
		suite.Run(testCase.name, func() {
			mockClient := &nuclioclientmock.Client{}
			var getErr error
			if testCase.existing == nil {
				getErr = apierrors.NewNotFound(schema.GroupResource{}, testProject)
			}
			mockClient.On("GetNuclioProject", mock.Anything, testNamespace, testProject).Return(testCase.existing, getErr)
			if testCase.mockWrites != nil {
				testCase.mockWrites(mockClient)
			}

			client := suite.newClient(mockClient)
			state, err := client.CommitCreate(context.TODO(), &platform.CommitCreateProjectOptions{
				Meta: platform.ProjectMeta{Name: testProject, Namespace: testNamespace},
				OpID: testCase.opID,
			})

			if testCase.expectedError {
				suite.Require().Error(err)
				var statusErr nuclio.WithStatusCode
				suite.Require().True(errors.As(err, &statusErr))
				suite.Require().Equal(testCase.expectedCode, statusErr.StatusCode())
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(testCase.expectedState, state)
			}
			mockClient.AssertExpectations(suite.T())
		})
	}
}

func (suite *FollowerTestSuite) TestCommitUpdate() {
	for _, testCase := range []struct {
		name          string
		existing      *nuclioio.NuclioProject
		mockWrites    func(*nuclioclientmock.Client)
		owner         string
		opID          string
		prevOpID      string
		expectedState *platform.Project2PCState
		expectedError bool
		expectedCode  int
	}{
		{
			name:     "HappyPathAdvancesSpecAndOpID",
			existing: newProjectFixture("op-1", leaderCommon.OrisSyncStatusOnline),
			mockWrites: func(m *nuclioclientmock.Client) {
				m.On("UpdateNuclioProject", mock.Anything, testNamespace, mock.MatchedBy(func(p *nuclioio.NuclioProject) bool {
					return p.Labels[leaderCommon.OrisLabelKeyOpID] == "op-2" &&
						p.Labels[leaderCommon.OrisLabelKeySyncStatus] == string(leaderCommon.OrisSyncStatusOnline) &&
						p.Spec.Owner == "new-owner"
				})).Return(&nuclioio.NuclioProject{}, nil).Once()
			},
			owner:         "new-owner",
			opID:          "op-2",
			prevOpID:      "op-1",
			expectedState: &platform.Project2PCState{Name: testProject, OpID: "op-2", SyncStatus: "online"},
		},
		{
			name:          "IdempotentReplay",
			existing:      newProjectFixture("op-2", leaderCommon.OrisSyncStatusOnline),
			opID:          "op-2",
			prevOpID:      "op-1",
			expectedState: &platform.Project2PCState{Name: testProject, OpID: "op-2", SyncStatus: "online"},
		},
		{
			name:          "RejectedWhenNotFound",
			opID:          "op-1",
			prevOpID:      "",
			expectedError: true,
		},
		{
			name:          "RejectedOnCASMismatch",
			existing:      newProjectFixture("op-1", leaderCommon.OrisSyncStatusOnline),
			opID:          "op-2",
			prevOpID:      "wrong-prev-op",
			expectedError: true,
			expectedCode:  409,
		},
		{
			name:          "RejectedOnStaleOpIDOrdering",
			existing:      newProjectFixture("op-2", leaderCommon.OrisSyncStatusOnline),
			opID:          "op-1",
			prevOpID:      "op-2",
			expectedError: true,
			expectedCode:  409,
		},
	} {
		suite.Run(testCase.name, func() {
			mockClient := &nuclioclientmock.Client{}
			var getErr error
			if testCase.existing == nil {
				getErr = apierrors.NewNotFound(schema.GroupResource{}, testProject)
			}
			mockClient.On("GetNuclioProject", mock.Anything, testNamespace, testProject).Return(testCase.existing, getErr)
			if testCase.mockWrites != nil {
				testCase.mockWrites(mockClient)
			}

			client := suite.newClient(mockClient)
			state, err := client.CommitUpdate(context.TODO(), &platform.CommitUpdateProjectOptions{
				ProjectConfig: platform.ProjectConfig{
					Meta: platform.ProjectMeta{Name: testProject, Namespace: testNamespace},
					Spec: platform.ProjectSpec{Owner: testCase.owner},
				},
				OpID:     testCase.opID,
				PrevOpID: testCase.prevOpID,
			})

			if testCase.expectedError {
				suite.Require().Error(err)
				if testCase.expectedCode != 0 {
					var statusErr nuclio.WithStatusCode
					suite.Require().True(errors.As(err, &statusErr))
					suite.Require().Equal(testCase.expectedCode, statusErr.StatusCode())
				}
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(testCase.expectedState, state)
			}
			mockClient.AssertExpectations(suite.T())
		})
	}
}

func (suite *FollowerTestSuite) TestPrepareDelete() {
	for _, testCase := range []struct {
		name          string
		existing      *nuclioio.NuclioProject
		mockWrites    func(*nuclioclientmock.Client)
		opID          string
		prevOpID      string
		expectedState *platform.Project2PCState
		expectedError bool
		expectedCode  int
	}{
		{
			name:          "IdempotentWhenAlreadyGone",
			opID:          "op-1",
			expectedState: &platform.Project2PCState{Name: testProject, OpID: "op-1", SyncStatus: "deleting"},
		},
		{
			name:     "HappyPathFlipsToDeleting",
			existing: newProjectFixture("op-1", leaderCommon.OrisSyncStatusOnline),
			mockWrites: func(m *nuclioclientmock.Client) {
				m.On("UpdateNuclioProject", mock.Anything, testNamespace, mock.MatchedBy(func(p *nuclioio.NuclioProject) bool {
					return p.Labels[leaderCommon.OrisLabelKeyOpID] == "op-2" &&
						p.Labels[leaderCommon.OrisLabelKeySyncStatus] == string(leaderCommon.OrisSyncStatusDeleting)
				})).Return(&nuclioio.NuclioProject{}, nil).Once()
			},
			opID:          "op-2",
			prevOpID:      "op-1",
			expectedState: &platform.Project2PCState{Name: testProject, OpID: "op-2", SyncStatus: "deleting"},
		},
		{
			name:          "IdempotentReplaySameOpID",
			existing:      newProjectFixture("op-2", leaderCommon.OrisSyncStatusDeleting),
			opID:          "op-2",
			prevOpID:      "op-1",
			expectedState: &platform.Project2PCState{Name: testProject, OpID: "op-2", SyncStatus: "deleting"},
		},
		{
			name:          "ConflictOnDifferentConcurrentDelete",
			existing:      newProjectFixture("op-2", leaderCommon.OrisSyncStatusDeleting),
			opID:          "op-3",
			prevOpID:      "op-1",
			expectedError: true,
			expectedCode:  409,
		},
		{
			name:          "RejectedOnCASMismatch",
			existing:      newProjectFixture("op-1", leaderCommon.OrisSyncStatusOnline),
			opID:          "op-2",
			prevOpID:      "wrong-prev-op",
			expectedError: true,
			expectedCode:  409,
		},
	} {
		suite.Run(testCase.name, func() {
			mockClient := &nuclioclientmock.Client{}
			var getErr error
			if testCase.existing == nil {
				getErr = apierrors.NewNotFound(schema.GroupResource{}, testProject)
			}
			mockClient.On("GetNuclioProject", mock.Anything, testNamespace, testProject).Return(testCase.existing, getErr)
			if testCase.mockWrites != nil {
				testCase.mockWrites(mockClient)
			}

			client := suite.newClient(mockClient)
			state, err := client.PrepareDelete(context.TODO(), &platform.PrepareDeleteProjectOptions{
				Meta:     platform.ProjectMeta{Name: testProject, Namespace: testNamespace},
				OpID:     testCase.opID,
				PrevOpID: testCase.prevOpID,
			})

			if testCase.expectedError {
				suite.Require().Error(err)
				var statusErr nuclio.WithStatusCode
				suite.Require().True(errors.As(err, &statusErr))
				suite.Require().Equal(testCase.expectedCode, statusErr.StatusCode())
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(testCase.expectedState, state)
			}
			mockClient.AssertExpectations(suite.T())
		})
	}
}

func (suite *FollowerTestSuite) TestCommitDelete() {
	for _, testCase := range []struct {
		name          string
		existing      *nuclioio.NuclioProject
		mockWrites    func(*nuclioclientmock.Client)
		opID          string
		expectedState *platform.Project2PCState
		expectedError bool
		expectedCode  int
	}{
		{
			name:          "IdempotentWhenAlreadyGone",
			opID:          "op-1",
			expectedState: &platform.Project2PCState{Name: testProject, OpID: "op-1"},
		},
		{
			name:     "HappyPathRemovesCRD",
			existing: newProjectFixture("op-2", leaderCommon.OrisSyncStatusDeleting),
			mockWrites: func(m *nuclioclientmock.Client) {
				m.On("DeleteNuclioProject", mock.Anything, testNamespace, testProject, metav1.DeleteOptions{}).Return(nil).Once()
			},
			opID:          "op-2",
			expectedState: &platform.Project2PCState{Name: testProject, OpID: "op-2"},
		},
		{
			name:          "RejectedWhenNotInDeletingState",
			existing:      newProjectFixture("op-1", leaderCommon.OrisSyncStatusOnline),
			opID:          "op-1",
			expectedError: true,
			expectedCode:  409,
		},
		{
			name:          "RejectedOnOpIDMismatch",
			existing:      newProjectFixture("op-2", leaderCommon.OrisSyncStatusDeleting),
			opID:          "op-3",
			expectedError: true,
			expectedCode:  409,
		},
	} {
		suite.Run(testCase.name, func() {
			mockClient := &nuclioclientmock.Client{}
			var getErr error
			if testCase.existing == nil {
				getErr = apierrors.NewNotFound(schema.GroupResource{}, testProject)
			}
			mockClient.On("GetNuclioProject", mock.Anything, testNamespace, testProject).Return(testCase.existing, getErr)
			if testCase.mockWrites != nil {
				testCase.mockWrites(mockClient)
			}

			client := suite.newClient(mockClient)
			state, err := client.CommitDelete(context.TODO(), &platform.CommitDeleteProjectOptions{
				Meta: platform.ProjectMeta{Name: testProject, Namespace: testNamespace},
				OpID: testCase.opID,
			})

			if testCase.expectedError {
				suite.Require().Error(err)
				var statusErr nuclio.WithStatusCode
				suite.Require().True(errors.As(err, &statusErr))
				suite.Require().Equal(testCase.expectedCode, statusErr.StatusCode())
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(testCase.expectedState, state)
			}
			mockClient.AssertExpectations(suite.T())
		})
	}
}

func (suite *FollowerTestSuite) TestList() {
	for _, testCase := range []struct {
		name      string
		listItems []nuclioio.NuclioProject
		run       func(client project.Client)
	}{
		{
			name: "FiltersSortsAndPages",
			listItems: []nuclioio.NuclioProject{
				*namedProjectFixture("proj-a", "op-1", leaderCommon.OrisSyncStatusCreating),
				*namedProjectFixture("proj-b", "op-1", leaderCommon.OrisSyncStatusCreating),
				*namedProjectFixture("proj-c", "op-1", leaderCommon.OrisSyncStatusCreating),
			},
			run: func(client project.Client) {
				firstPage, err := client.List(context.TODO(), &platform.ListProjectStatesOptions{
					Namespace: testNamespace,
					Limit:     2,
				})
				suite.Require().NoError(err)
				suite.Require().Len(firstPage.States, 2)
				suite.Require().Equal("proj-a", firstPage.States[0].Name)
				suite.Require().Equal("proj-b", firstPage.States[1].Name)
				suite.Require().Equal("proj-b", firstPage.NextCursor)

				secondPage, err := client.List(context.TODO(), &platform.ListProjectStatesOptions{
					Namespace: testNamespace,
					Cursor:    firstPage.NextCursor,
					Limit:     2,
				})
				suite.Require().NoError(err)
				suite.Require().Len(secondPage.States, 1)
				suite.Require().Equal("proj-c", secondPage.States[0].Name)
				suite.Require().Empty(secondPage.NextCursor)
			},
		},
		{
			name:      "FiltersByUpdatedAfter",
			listItems: []nuclioio.NuclioProject{*newProjectFixture("op-1", leaderCommon.OrisSyncStatusCreating)},
			run: func(client project.Client) {
				cutoff := testUpdatedAt.Add(time.Hour)
				page, err := client.List(context.TODO(), &platform.ListProjectStatesOptions{
					Namespace:    testNamespace,
					UpdatedAfter: &cutoff,
				})
				suite.Require().NoError(err)
				suite.Require().Empty(page.States)
			},
		},
	} {
		suite.Run(testCase.name, func() {
			mockClient := &nuclioclientmock.Client{}
			mockClient.On("ListNuclioProjects", mock.Anything, testNamespace, metav1.ListOptions{}).
				Return(&nuclioio.NuclioProjectList{Items: testCase.listItems}, nil)

			client := suite.newClient(mockClient)
			testCase.run(client)

			mockClient.AssertExpectations(suite.T())
		})
	}
}

// --- test helpers (private, below public functions) ---

// newClient builds a follower Client backed by the given mock.
func (suite *FollowerTestSuite) newClient(nuclioClientSet nuclioclient.Client) project.Client {
	client, err := NewClient(suite.logger, nil, &nuclioclient.Consumer{NuclioClientSet: nuclioClientSet})
	suite.Require().NoError(err)
	return client
}

// namedProjectFixture builds an in-memory NuclioProject stamped with the given op_id/sync-status
// labels, used only as mock return data - it is never persisted anywhere.
func namedProjectFixture(name, opID string, syncStatus leaderCommon.OrisSyncStatus) *nuclioio.NuclioProject {
	return &nuclioio.NuclioProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels: map[string]string{
				leaderCommon.OrisLabelKeyOpID:       opID,
				leaderCommon.OrisLabelKeySyncStatus: string(syncStatus),
			},
		},
		Status: platform.ProjectStatus{UpdatedAt: &testUpdatedAt},
	}
}

// newProjectFixture is namedProjectFixture for testProject, the name every test case but
// TestList's FiltersSortsAndPages uses.
func newProjectFixture(opID string, syncStatus leaderCommon.OrisSyncStatus) *nuclioio.NuclioProject {
	return namedProjectFixture(testProject, opID, syncStatus)
}

func TestFollowerTestSuite(t *testing.T) {
	suite.Run(t, new(FollowerTestSuite))
}
