/*
Copyright 2017 The Nuclio Authors.

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

package functiontemplates

import (
	"archive/zip"
	"bytes"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/logger"
)

type ZipFunctionTemplateFetcher struct {
	BaseFunctionTemplateFetcher

	fileAddress string
	logger      logger.Logger
}

func NewZipFunctionTemplateFetcher(parentLogger logger.Logger, fileAddress string) (*ZipFunctionTemplateFetcher, error) {

	return &ZipFunctionTemplateFetcher{
		fileAddress: fileAddress,
		logger:      parentLogger.GetChild("ZipFunctionTemplateFetcher"),
	}, nil
}

func (zftf *ZipFunctionTemplateFetcher) Fetch() ([]*FunctionTemplate, error) {
	var functionTemplates []*FunctionTemplate
	var functionsFileContents map[string]*FunctionTemplateFileContents
	var zipFileBody []byte
	var err error

	zftf.logger.DebugWith("Getting the zip file from the given address", "fileAddress", zftf.fileAddress)

	if common.IsLocalFileURL(zftf.fileAddress) {
		localFilePath := common.GetPathFromLocalFileURL(zftf.fileAddress)
		zipFileBody, err = ioutil.ReadFile(localFilePath)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to read local file")
		}

	} else {

		response, err := http.Get(zftf.fileAddress)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get zip file")
		}
		defer response.Body.Close() // nolint: errcheck

		zipFileBody, err = ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to read response body")
		}
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipFileBody), int64(len(zipFileBody)))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create zip reader")
	}

	functionsFileContents = zftf.parseFiles(zipReader)

	// parse every function file contents into a function template object
	for functionName, ffc := range functionsFileContents {
		functionTemplate, err := zftf.createFunctionTemplate(*ffc, functionName)
		if err != nil {
			zftf.logger.WarnWith("Failed to create function template",
				"functionName",
				functionName,
				"err",
				err)
		}

		if functionTemplate != nil {
			functionTemplates = append(functionTemplates, functionTemplate)
		}
	}

	return functionTemplates, nil
}

func (zftf *ZipFunctionTemplateFetcher) parseFiles(zipReader *zip.Reader) map[string]*FunctionTemplateFileContents {
	functionTemplateFileContents := make(map[string]*FunctionTemplateFileContents)

	// Read all the files from zip archive
	for _, zipFile := range zipReader.File {
		fileName := zipFile.Name
		splitFileName := strings.Split(fileName, "/")

		// make sure it's a file inside a valid function folder path and it's not a folder
		if zipFile.FileInfo().IsDir() || len(splitFileName) != 3 {
			continue
		}

		functionName := splitFileName[1]
		fileNameWithoutPath := splitFileName[2]

		// get file contents
		fileContents, err := common.GetZipFileContents(zipFile)
		if err != nil {
			zftf.logger.WarnWith("Failed to get zip file contents. Ignoring file", "fileName", fileName, "err", err)
			continue
		}

		// if functionTemplateFileContents was not created for this function - create it
		if _, ok := functionTemplateFileContents[functionName]; !ok {
			zftf.logger.Debug("Found new function files", "functionName", functionName)

			functionTemplateFileContents[functionName] = &FunctionTemplateFileContents{}
		}
		fs := functionTemplateFileContents[functionName]

		if strings.Contains(fileNameWithoutPath, ".values") {
			fs.Values = fileContents

		} else if strings.Contains(fileNameWithoutPath, ".template") {
			fs.Template = fileContents

		} else {
			fs.Code = fileContents
		}
	}

	return functionTemplateFileContents
}
