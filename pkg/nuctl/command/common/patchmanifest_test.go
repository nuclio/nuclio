//go:build test_unit

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

package common

import (
	"context"
	"os"
	"testing"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/stretchr/testify/suite"
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
	testData := "{\"success\":[\"success1\",\"success2\"]," +
		"\"skipped\":[\"skipped1\",\"skipped2\"]," +
		"\"failed\":{\"failed1\":{\"error\":\"error1\",\"retryable\":false}," +
		"\"failed2\":{\"error\":\"error2\",\"retryable\":true}}}"
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

	manifest, err := NewPatchManifestFromFile(tempFile.Name())
	suite.Require().NoError(err)
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

	suite.Require().Equal("{\"success\":[\"success1\",\"success2\"],"+
		"\"skipped\":[\"skipped1\",\"skipped2\"],"+
		"\"failed\":{\"failed1\":{\"error\":\"error1\",\"retryable\":false},"+
		"\"failed2\":{\"error\":\"error2\",\"retryable\":true}}}", string(manifestData))

}

func TestPatchManifesTestSuite(t *testing.T) {
	suite.Run(t, new(PatchManifestTestSuite))
}
