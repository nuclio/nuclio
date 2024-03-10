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

package util

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type unarchiverTestSuite struct {
	suite.Suite
	unarchiver  *Unarchiver
	logger      logger.Logger
	ctx         context.Context
	tempDir     string
	archivesDir string
}

func (suite *unarchiverTestSuite) SetupTest() {
	var err error

	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err, "Failed to create logger")

	suite.ctx = context.Background()

	suite.tempDir, err = os.MkdirTemp("", "*")
	suite.Require().NoError(err, "Failed to create temp dir")

	suite.unarchiver, err = NewUnarchiver(suite.logger)
	suite.Require().NoError(err, "Failed to create unarchiver")

	suite.archivesDir = path.Join(common.GetSourceDir(), "test", "_archives")
}

func (suite *unarchiverTestSuite) TearDownTest() {
	// remove temp dir
	suite.Require().NoError(os.RemoveAll(suite.tempDir))
}

func (suite *unarchiverTestSuite) TestExtractArchive() {
	// get the zip file path from the archives dir
	zipFilePath := path.Join(suite.archivesDir, "test.zip")

	// extract the zip file to a target dir
	targetDirPath := path.Join(suite.tempDir, "test-target-path")
	err := suite.unarchiver.Extract(suite.ctx, zipFilePath, targetDirPath)
	suite.Require().NoError(err, "Failed to extract archive")

	// list files in target dir
	files, err := os.ReadDir(targetDirPath)
	suite.Require().NoError(err, "Failed to read dir")

	// test.zip should contain a folder with 3 files, and a base-file on root level
	subdirName := "random-dir"
	baseFileName := "base-file"
	suite.Require().Len(files, 2, "Unexpected number of files in target dir")

	// assert that the files in the target subdir are the same as the files in the temp subdir
	for _, file := range files {
		suite.Require().Contains([]string{subdirName, baseFileName}, file.Name(), "File not found in target dir")
	}

	// list files in the subdir
	files, err = os.ReadDir(path.Join(targetDirPath, subdirName))
	suite.Require().NoError(err, "Failed to read dir")

	// the subdir contains 3 random files
	var fileNames []string
	for i := 0; i < 3; i++ {
		fileName := fmt.Sprintf("random-file-%d", i)
		fileNames = append(fileNames, fileName)
	}

	// assert that the files in the target subdir are the same as the files in the temp subdir
	for _, file := range files {
		suite.Require().Contains(fileNames, file.Name(), "File not found in target dir")
	}
}

func (suite *unarchiverTestSuite) TestIsArchive() {
	for _, testCase := range []struct {
		name               string
		expectedCompressed bool
	}{
		{
			name:               "zip",
			expectedCompressed: true,
		},
		{
			name:               "tar",
			expectedCompressed: true,
		},
		{
			name:               "7z",
			expectedCompressed: true,
		},
		{
			name:               "rar",
			expectedCompressed: true,
		},
		{
			name:               "tar.gz",
			expectedCompressed: true,
		},
		{
			name:               "tar.bz2",
			expectedCompressed: true,
		},
		{
			name:               "tar.xz",
			expectedCompressed: true,
		},
		{
			name:               "tar.lzma",
			expectedCompressed: true,
		},
		{
			name:               "tar.sz",
			expectedCompressed: true,
		},
		// false cases
		{
			name: "txt",
		},
		{
			name: "jar",
		},
		{
			name: "json",
		},
		{
			name: "random",
		},
	} {
		suite.Run(testCase.name, func() {
			fileName := fmt.Sprintf("test.%s", testCase.name)
			filePath := path.Join(suite.archivesDir, fileName)

			compressed := IsArchive(filePath)
			suite.Require().Equal(testCase.expectedCompressed, compressed)
		})
	}
}

func TestUnarchiverTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(unarchiverTestSuite))
}
