package test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/v3io/v3io-go/pkg/controlplane"
	"github.com/v3io/v3io-go/pkg/controlplane/http"
	"github.com/v3io/v3io-go/pkg/errors"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type githubClientSuite struct {
	suite.Suite
	logger  logger.Logger
	session v3ioc.Session
	userID  string
	ctx     context.Context
}

func (suite *githubClientSuite) SetupSuite() {
	controlplaneURL := os.Getenv("V3IO_CONTROLPLANE_URL")
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")

	// create a security admin session
	newSessionInput := v3ioc.NewSessionInput{}
	newSessionInput.Username = os.Getenv("V3IO_CONTROLPLANE_USERNAME")
	newSessionInput.Password = os.Getenv("V3IO_CONTROLPLANE_PASSWORD")
	newSessionInput.Endpoints = []string{controlplaneURL}

	session, err := v3iochttp.NewSession(suite.logger, &newSessionInput)
	suite.Require().NoError(err, fmt.Sprintf("\nInput: %v\n", newSessionInput))

	// create a unique user for the tests
	ts := time.Now().Unix()
	createUserInput := v3ioc.CreateUserInput{}
	createUserInput.Ctx = suite.ctx
	createUserInput.FirstName = fmt.Sprintf("Test-%d", ts)
	createUserInput.LastName = fmt.Sprintf("User-%d", ts)
	createUserInput.Username = fmt.Sprintf("testuser-%d", ts)
	createUserInput.Password = fmt.Sprintf("testpasswd-%d", ts)
	createUserInput.Email = fmt.Sprintf("testuser-%d@user.com", ts)
	createUserInput.Description = "A user created from tests"
	createUserInput.AssignedPolicies = []string{"Security Admin", "Data", "Application Admin", "Function Admin"}

	// create a user with security session
	createUserOutput, err := session.CreateUserSync(&createUserInput)
	suite.Require().NoError(err)
	suite.Require().NotNil(createUserOutput.ID)
	suite.userID = createUserOutput.ID

	// create a session with that user
	newSessionInput.Username = createUserInput.Username
	newSessionInput.Password = createUserInput.Password
	newSessionInput.Endpoints = []string{controlplaneURL}

	suite.session, err = v3iochttp.NewSession(suite.logger, &newSessionInput)
	suite.Require().NoError(err)

	time.Sleep(30 * time.Second)
}

func (suite *githubClientSuite) TearDownSuite() {
	deleteUserInput := v3ioc.DeleteUserInput{}
	deleteUserInput.ID = suite.userID

	err := suite.session.DeleteUserSync(&deleteUserInput)
	suite.Require().NoError(err)
}

func (suite *githubClientSuite) SetupTest() {
	suite.ctx = context.WithValue(nil, "RequestID", "test-0")
}

func (suite *githubClientSuite) TestCreateContainerStringID() {
	createContainerInput := v3ioc.CreateContainerInput{}
	createContainerInput.Ctx = suite.ctx
	createContainerInput.Name = "container-string"

	createContainerOutput, err := suite.session.CreateContainerSync(&createContainerInput)
	suite.Require().NoError(err)
	suite.Require().NotEqual(0, createContainerOutput.IDNumeric)

	time.Sleep(5 * time.Second)

	deleteContainerInput := v3ioc.DeleteContainerInput{}
	deleteContainerInput.Ctx = suite.ctx
	deleteContainerInput.IDNumeric = createContainerOutput.IDNumeric

	err = suite.session.DeleteContainerSync(&deleteContainerInput)
	suite.Require().NoError(err)
}

func (suite *githubClientSuite) TestCreateContainerNumericID() {
	createContainerInput := v3ioc.CreateContainerInput{}
	createContainerInput.Ctx = suite.ctx
	createContainerInput.IDNumeric = 300
	createContainerInput.Name = "container-int"

	createContainerOutput, err := suite.session.CreateContainerSync(&createContainerInput)
	suite.Require().NoError(err)
	suite.Require().Equal(createContainerInput.IDNumeric, createContainerOutput.IDNumeric)

	time.Sleep(5 * time.Second)

	deleteContainerInput := v3ioc.DeleteContainerInput{}
	deleteContainerInput.Ctx = suite.ctx
	deleteContainerInput.IDNumeric = createContainerOutput.IDNumeric

	err = suite.session.DeleteContainerSync(&deleteContainerInput)
	suite.Require().NoError(err)
}

func (suite *githubClientSuite) TestCreateSessionWithTimeout() {

	// create a security admin session
	newSessionInput := v3ioc.NewSessionInput{}
	newSessionInput.Username = os.Getenv("V3IO_CONTROLPLANE_USERNAME")
	newSessionInput.Password = os.Getenv("V3IO_CONTROLPLANE_PASSWORD")
	newSessionInput.Endpoints = []string{os.Getenv("V3IO_CONTROLPLANE_URL")}
	newSessionInput.Timeout = 1 * time.Millisecond

	session, err := v3iochttp.NewSession(suite.logger, &newSessionInput)
	suite.Require().Equal(v3ioerrors.ErrTimeout, err)
	suite.Require().Nil(session)
}

func (suite *githubClientSuite) TestCreateSessionWithBadPassword() {

	// create a security admin session
	newSessionInput := v3ioc.NewSessionInput{}
	newSessionInput.Username = os.Getenv("V3IO_CONTROLPLANE_USERNAME")
	newSessionInput.Password = "WRONG"
	newSessionInput.Endpoints = []string{os.Getenv("V3IO_CONTROLPLANE_URL")}

	session, err := v3iochttp.NewSession(suite.logger, &newSessionInput)
	suite.Equal(401, err.(v3ioerrors.ErrorWithStatusCode).StatusCode())
	suite.Require().Nil(session)
}

func (suite *githubClientSuite) TestCreateEventUsingAccessKey() {

	// Create new access key
	createAccessKeyInput := v3ioc.CreateAccessKeyInput{}
	createAccessKeyInput.Ctx = suite.ctx
	createAccessKeyInput.Label = "test_access_key_label"
	createAccessKeyInput.Plane = v3ioc.ControlPlane

	createAccessKeyOutput, err := suite.session.CreateAccessKeySync(&createAccessKeyInput)
	suite.Require().NoError(err)
	suite.Require().Equal(createAccessKeyOutput.Label, createAccessKeyInput.Label)

	// Create new session from access key
	newSessionInput := v3ioc.NewSessionInput{}
	newSessionInput.AccessKey = createAccessKeyOutput.ID
	newSessionInput.Endpoints = []string{os.Getenv("V3IO_CONTROLPLANE_URL")}
	accessKeySession, err := v3iochttp.NewSession(suite.logger, &newSessionInput)
	suite.Require().NoError(err)

	// Emit event
	createEventInput := v3ioc.CreateEventInput{}
	createEventInput.Ctx = suite.ctx
	createEventInput.Kind = "AppService.Test.Event"
	createEventInput.Source = "DummyService"

	err = accessKeySession.CreateEventSync(&createEventInput)
	suite.Require().NoError(err)

	// Delete access key
	deleteAccessKeyInput := v3ioc.DeleteAccessKeyInput{}
	deleteAccessKeyInput.ID = createAccessKeyOutput.ID
	deleteAccessKeyInput.Ctx = suite.ctx
	err = suite.session.DeleteAccessKeySync(&deleteAccessKeyInput)
	suite.Require().NoError(err)
}

func TestGithubClientTestSuite(t *testing.T) {
	suite.Run(t, new(githubClientSuite))
}
