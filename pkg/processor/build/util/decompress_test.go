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
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type decompressTestSuite struct {
	suite.Suite
	decompressor *Decompressor
	logger       logger.Logger
	ctx          context.Context
	tempDir      string
	archivesDir  string
}

func (suite *decompressTestSuite) SetupTest() {
	var err error

	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err, "Failed to create logger")

	suite.ctx = context.Background()

	suite.tempDir, err = os.MkdirTemp("", "*")
	suite.Require().NoError(err, "Failed to create temp dir")

	suite.decompressor, err = NewDecompressor(suite.logger)
	suite.Require().NoError(err, "Failed to create decompressor")

	suite.archivesDir = path.Join(common.GetSourceDir(), "test", "_archives")
}

func (suite *decompressTestSuite) TearDownTest() {
	// remove temp dir
	suite.Require().NoError(os.RemoveAll(suite.tempDir))
}

func (suite *decompressTestSuite) TestExtractArchive() {
	// create random files
	var filePaths []string
	var fileNames []string
	for i := 0; i < 3; i++ {
		fileName := fmt.Sprintf("random-file-%d", i)
		randomFilePath := path.Join(suite.tempDir, fileName)
		fileNames = append(fileNames, fileName)
		filePaths = append(filePaths, randomFilePath)
		err := os.WriteFile(randomFilePath, []byte("test"), 0600)
		suite.Require().NoError(err, "Failed to create random file")
	}

	// create zip file that contains the random files
	zipFilePath := path.Join(suite.tempDir, "test.zip")
	err := suite.createZipArchive(zipFilePath, filePaths)
	suite.Require().NoError(err, "Failed to create zip file")

	// extract the zip file to a target dir
	targetDirPath := path.Join(suite.tempDir, "test-target-path")
	err = suite.decompressor.ExtractArchive(suite.ctx, zipFilePath, targetDirPath)
	suite.Require().NoError(err, "Failed to extract archive")

	// list files in target dir
	files, err := os.ReadDir(targetDirPath)
	suite.Require().NoError(err, "Failed to read dir")
	suite.logger.DebugWith("Files in target dir", "files", files)

	// assert that the files in the target dir are the same as the files in the temp dir
	for _, file := range files {
		suite.Require().Contains(fileNames, file.Name(), "File not found in target dir")
	}
}

func (suite *decompressTestSuite) TestIsCompressed() {
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

			compressed := IsCompressed(filePath)
			suite.Require().Equal(testCase.expectedCompressed, compressed)
		})
	}
}

func (suite *decompressTestSuite) createZipArchive(targetPath string, filesToArchive []string) error {
	suite.logger.DebugWith("creating zip archive", "targetPath", targetPath, "filesToArchive", filesToArchive)
	archive, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	zipWriter := zip.NewWriter(archive)
	defer archive.Close()   // nolint: errcheck
	defer zipWriter.Close() // nolint: errcheck

	writeFileToZip := func(zipWriterInstance *zip.Writer, fileName string) error {
		suite.logger.DebugWith("Opening file", "fileName", fileName)
		file, err := os.Open(fileName)
		if err != nil {
			return err
		}
		defer file.Close() // nolint: errcheck
		suite.logger.DebugWith("Writing file to archive", "fileName", fileName)
		writer, err := zipWriterInstance.Create(filepath.Base(fileName))
		if err != nil {
			return err
		}
		if _, err := io.Copy(writer, file); err != nil {
			return err
		}
		return nil
	}

	for _, fileToArchive := range filesToArchive {
		if err := writeFileToZip(zipWriter, fileToArchive); err != nil {
			return err
		}
	}
	return nil
}

func TestDecompressTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(decompressTestSuite))
}
