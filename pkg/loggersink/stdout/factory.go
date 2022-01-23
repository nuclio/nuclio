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

package stdout

import (
	"io"
	"os"

	"github.com/nuclio/nuclio/pkg/loggersink"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
)

type factory struct{}

func (f *factory) Create(name string,
	loggerSinkConfiguration *platformconfig.LoggerSinkWithLevel) (logger.Logger, error) {

	var writer io.Writer = os.Stdout
	if redactingLogger := loggerSinkConfiguration.GetRedactingLogger(); redactingLogger != nil {

		// default redacting logger output to stdout
		if redactingLogger.GetOutput() == nil {
			redactingLogger.SetOutput(writer)
		}

		writer = redactingLogger
	}

	configuration, err := NewConfiguration(name, loggerSinkConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create prometheus pull configuration")
	}

	var level nucliozap.Level

	switch configuration.Level {
	case logger.LevelInfo:
		level = nucliozap.InfoLevel
	case logger.LevelWarn:
		level = nucliozap.WarnLevel
	case logger.LevelError:
		level = nucliozap.ErrorLevel
	default:
		level = nucliozap.DebugLevel
	}

	// get the default encoding and override line ending to newline
	encoderConfig := nucliozap.NewEncoderConfig()
	encoderConfig.JSON.LineEnding = "\n"
	encoderConfig.JSON.VarGroupName = configuration.VarGroupName
	encoderConfig.JSON.VarGroupMode = configuration.VarGroupMode
	encoderConfig.JSON.TimeFieldName = configuration.TimeFieldName
	encoderConfig.JSON.TimeFieldEncoding = configuration.TimeFieldEncoding

	return nucliozap.NewNuclioZap(name,
		configuration.Encoding,
		encoderConfig,
		writer,
		writer,
		level)
}

// register factory
func init() {
	loggersink.RegistrySingleton.Register("stdout", &factory{})
}
