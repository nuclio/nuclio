package common

import (
	"context"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"testing"

	"github.com/stretchr/testify/suite"
	"os"
)

type PatchManifestTestSuite struct {
	suite.Suite
	tempDir string
	logger  logger.Logger
	ctx     context.Context
}

func (suite *PatchManifestTestSuite) SetupSuite() {
	var err error

	suite.tempDir, err = os.MkdirTemp("", "patchManifest-test")
	suite.Require().NoError(err)
}

func (suite *PatchManifestTestSuite) TearDownSuite() {
	defer os.RemoveAll(suite.tempDir)
}

func (suite *PatchManifestTestSuite) TestNewPatchManifestFromFile() {
	testData := "{\"success\":[\"success1\",\"success2\"],\"skipped\":[\"skipped1\",\"skipped2\"],\"failed\":{\"failed1\":{\"error\":\"error1\",\"retryable\":false},\"failed2\":{\"error\":\"error2\",\"retryable\":true}}}"
	expected := NewPatchManifest()
	expected.Failed = map[string]FailDescription{
		"failed1": {Err: "error1", Retryable: false},
		"failed2": {Err: "error2", Retryable: true},
	}
	expected.Skipped = []string{
		"skipped1", "skipped2",
	}
	expected.Success = []string{
		"success1", "success2",
	}
	tempFile, err := os.CreateTemp(suite.tempDir, "test")
	suite.Require().NoError(err)
	_, err = tempFile.Write([]byte(testData))
	suite.Require().NoError(err)

	manifest := NewPatchManifestFromFile(tempFile.Name())
	suite.Require().Equal(expected.Failed, manifest.Failed, "Failed deployments was read incorrectly")
	suite.Require().Equal(expected.Success, manifest.Success, "Success deployments was read incorrectly")
	suite.Require().Equal(expected.Skipped, manifest.Skipped, "Skipped deployments was read incorrectly")
}

func (suite *PatchManifestTestSuite) TestPatchManifestSaveToFile() {
	manifest := NewPatchManifest()
	manifest.AddSkipped("skipped1")
	manifest.AddSkipped("skipped2")

	manifest.AddSuccess("success1")
	manifest.AddSuccess("success2")

	manifest.AddFailure("failed1", errors.New("error1"), false)
	manifest.AddFailure("failed2", errors.New("error2"), true)

	tempFile, err := os.CreateTemp(suite.tempDir, "test_save_file")
	suite.Require().NoError(err)

	manifest.SaveToFile(suite.ctx, suite.logger, tempFile.Name())

	manifestData, err := os.ReadFile(tempFile.Name())
	suite.Require().NoError(err)

	suite.Require().Equal("{\"success\":[\"success1\",\"success2\"],\"skipped\":[\"skipped1\",\"skipped2\"],\"failed\":{\"failed1\":{\"error\":\"error1\",\"retryable\":false},\"failed2\":{\"error\":\"error2\",\"retryable\":true}}}", string(manifestData))

}

func TestPatchManifesTestSuite(t *testing.T) {
	suite.Run(t, new(PatchManifestTestSuite))
}
