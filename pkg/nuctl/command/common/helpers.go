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
	"encoding/json"
	"io"
	"os"

	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"sigs.k8s.io/yaml"
)

func ReadFromInOrStdin(r io.Reader) ([]byte, error) {
	switch in := r.(type) {
	case *os.File:
		info, err := in.Stat()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to stat file")
		}

		// ensuring input piped or file
		if info.Mode()&os.ModeNamedPipe != 0 || info.Mode().IsRegular() {
			return io.ReadAll(r)
		}
	case io.Reader:
		return io.ReadAll(r)
	}
	return nil, nil
}

// OpenFile validates filepath existence and returns a file (it is the caller's responsibility to close it)
func OpenFile(filepath string) (*os.File, error) {
	fileInfo, err := os.Stat(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.Wrapf(err, "File path `%s` does not exists", filepath)
		}
		return nil, errors.Wrapf(err, "Failed to stat file `%s`", filepath)
	}
	if fileInfo.IsDir() {
		return nil, errors.Errorf("Expected path to a file, received a dir `%s`", filepath)
	}
	file, err := os.Open(filepath)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to open file `%s`", filepath)
	}
	return file, err
}

func GetUnmarshalFunc(bytes []byte) (func(data []byte, v interface{}) error, error) {
	var err error
	var obj map[string]interface{}

	if err = json.Unmarshal(bytes, &obj); err == nil {
		return json.Unmarshal, nil
	}

	if err = yaml.Unmarshal(bytes, &obj); err == nil {
		return func(data []byte, v interface{}) error {
			return yaml.Unmarshal(data, v)
		}, nil
	}

	return nil, errors.New("Input is neither json nor yaml")
}

// ConvertMapToFunctionConfigWithStatus converts a map to a function config with status
func ConvertMapToFunctionConfigWithStatus(functionMap map[string]interface{}) (*functionconfig.ConfigWithStatus, error) {
	var functionConfigWithStatus *functionconfig.ConfigWithStatus

	// convert to json
	functionConfigJSON, err := json.Marshal(functionMap)
	if err != nil {
		return functionConfigWithStatus, errors.Wrap(err, "Failed to marshal function config")
	}

	// convert to function config with status
	if err := json.Unmarshal(functionConfigJSON, &functionConfigWithStatus); err != nil {
		return functionConfigWithStatus, errors.Wrap(err, "Failed to unmarshal function config")
	}

	return functionConfigWithStatus, nil
}

// saveReportToFile saves the report to the file
// It does not return any errors if they occur; instead, it logs an error for the best effort
func saveReportToFile(ctx context.Context, loggerInstance logger.Logger, report interface{}, path string) {
	file, err := json.Marshal(report)
	if err != nil {
		loggerInstance.ErrorWithCtx(ctx,
			"Failed to marshal report to json",
			"err", err,
			"path", path)
	}
	if err := os.WriteFile(path, file, 0644); err != nil {
		loggerInstance.ErrorWithCtx(ctx,
			"Failed to write report to file",
			"err", err,
			"path", path)
	} else {
		loggerInstance.InfoWithCtx(ctx,
			"Saved import report",
			"reportPath", path)
	}
}
