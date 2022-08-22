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
package http

import (
	"os"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type fileResponse struct {
	logger        logger.Logger
	deleteOnClose bool
	path          string
	file          *os.File
}

func newFileResponse(logger logger.Logger, path string, deleteOnClose bool) (*fileResponse, error) {
	var err error

	newFileResponse := fileResponse{
		logger:        logger,
		path:          path,
		deleteOnClose: deleteOnClose,
	}

	// try to open the file for read
	newFileResponse.file, err = os.Open(path)
	if err != nil {
		return nil, err
	}

	return &newFileResponse, nil
}

func (fr *fileResponse) Read(p []byte) (n int, err error) {
	return fr.file.Read(p)
}

func (fr *fileResponse) Close() error {
	if err := fr.file.Close(); err != nil {
		return errors.Wrap(err, "Failed to close file")
	}

	if fr.deleteOnClose {
		if err := os.Remove(fr.path); err != nil {

			// just warn, don't return an error
			fr.logger.WarnWith("Failed to remove file after sending",
				"err", err.Error(),
				"path", fr.path)
		}
	}

	return nil
}
