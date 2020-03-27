package test

import (
	"os"

	"github.com/v3io/v3io-go/pkg/dataplane"
	"github.com/v3io/v3io-go/pkg/dataplane/http"
	"github.com/v3io/v3io-go/pkg/errors"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type testSuite struct { // nolint: deadcode
	suite.Suite
	logger              logger.Logger
	container           v3io.Container
	url                 string
	containerName       string
	authenticationToken string
	accessKey           string
}

func (suite *testSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
}

func (suite *testSuite) populateDataPlaneInput(dataPlaneInput *v3io.DataPlaneInput) {
	dataPlaneInput.URL = suite.url
	dataPlaneInput.ContainerName = suite.containerName
	dataPlaneInput.AuthenticationToken = suite.authenticationToken
	dataPlaneInput.AccessKey = suite.accessKey
}

func (suite *testSuite) createContext() {
	var err error

	// create a context
	suite.container, err = v3iohttp.NewContext(suite.logger, &v3iohttp.NewContextInput{})
	suite.Require().NoError(err)

	// populate fields that would have been populated by session/container
	suite.containerName = "bigdata"
	suite.url = os.Getenv("V3IO_DATAPLANE_URL")
	username := os.Getenv("V3IO_DATAPLANE_USERNAME")
	password := os.Getenv("V3IO_DATAPLANE_PASSWORD")

	if username != "" && password != "" {
		suite.authenticationToken = v3iohttp.GenerateAuthenticationToken(username, password)
	}

	suite.accessKey = os.Getenv("V3IO_DATAPLANE_ACCESS_KEY")
}

func (suite *testSuite) createContainer() {

	// create a context
	context, err := v3iohttp.NewContext(suite.logger, &v3iohttp.NewContextInput{})
	suite.Require().NoError(err)

	session, err := context.NewSession(&v3io.NewSessionInput{
		URL:       os.Getenv("V3IO_DATAPLANE_URL"),
		Username:  os.Getenv("V3IO_DATAPLANE_USERNAME"),
		Password:  os.Getenv("V3IO_DATAPLANE_PASSWORD"),
		AccessKey: os.Getenv("V3IO_DATAPLANE_ACCESS_KEY"),
	})
	suite.Require().NoError(err)

	suite.container, err = session.NewContainer(&v3io.NewContainerInput{
		ContainerName: "bigdata",
	})
	suite.Require().NoError(err)
}

type streamTestSuite struct { // nolint: deadcode
	testSuite
	testPath string
}

func (suite *streamTestSuite) SetupTest() {
	suite.testPath = "/stream-test"
	err := suite.deleteAllStreamsInPath(suite.testPath)

	// get the underlying root error
	if err != nil {
		errWithStatusCode, errHasStatusCode := err.(v3ioerrors.ErrorWithStatusCode)
		suite.Require().True(errHasStatusCode)

		// File not found is OK
		suite.Require().Equal(404, errWithStatusCode.StatusCode(), "Failed to setup test suite")
	}
}

func (suite *streamTestSuite) TearDownTest() {
	err := suite.deleteAllStreamsInPath(suite.testPath)
	suite.Require().NoError(err, "Failed to tear down test suite")
}

func (suite *streamTestSuite) deleteAllStreamsInPath(path string) error {
	getContainerContentsInput := v3io.GetContainerContentsInput{
		Path: path,
	}

	suite.populateDataPlaneInput(&getContainerContentsInput.DataPlaneInput)

	// get all streams in the test path
	response, err := suite.container.GetContainerContentsSync(&getContainerContentsInput)

	if err != nil {
		return err
	}
	response.Release()

	// iterate over streams (prefixes) and delete them
	for _, commonPrefix := range response.Output.(*v3io.GetContainerContentsOutput).CommonPrefixes {
		deleteStreamInput := v3io.DeleteStreamInput{
			Path: "/" + commonPrefix.Prefix,
		}

		suite.populateDataPlaneInput(&deleteStreamInput.DataPlaneInput)

		err := suite.container.DeleteStreamSync(&deleteStreamInput)
		if err != nil {
			return err
		}
	}

	return nil
}
