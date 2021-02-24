/*
Copyright 2021 The Nuclio Authors.

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

package loggerus

import (
	"io"
	"os"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/loggerus"
	"github.com/sirupsen/logrus"
)

func MuxLoggers(loggers ...logger.Logger) (*loggerus.MuxLogger, error) {
	return loggerus.NewMuxLogger(loggers...)
}

func CreateTestLogger(name string) (logger.Logger, error) {
	return loggerus.NewLoggerusForTests(name)
}

func CreateStdoutLogger(loggerName string,
	loggerLevel logger.Level,
	loggerFormatterKind LoggerFormatterKind,
	noColor bool) (logger.Logger, error) {
	return CreateCustomOutputLogger(loggerName,
		loggerLevel,
		loggerFormatterKind,
		os.Stdout,
		true,
		noColor)
}

func CreateCmdLogger(loggerName string, loggerLevel logger.Level) (logger.Logger, error) {
	return CreateStdoutLogger(loggerName, loggerLevel, LoggerFormatterKindText, false)
}

func CreateFileLogger(loggerName string,
	loggerLevel logger.Level,
	loggerFormatterKind LoggerFormatterKind,
	loggerFilePath string,
	noColor bool) (logger.Logger, error) {
	fileOutputFile, err := common.EnsureOpenFile(loggerFilePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to open logger output file")
	}

	return CreateCustomOutputLogger(loggerName,
		loggerLevel,
		loggerFormatterKind,
		fileOutputFile,
		false,
		noColor)
}

func CreateCustomOutputLogger(loggerName string,
	loggerLevel logger.Level,
	loggerFormatterKindText LoggerFormatterKind,
	output io.Writer,
	enrichWhoField bool,
	noColor bool) (logger.Logger, error) {

	switch loggerFormatterKindText {
	case LoggerFormatterKindText:
		return loggerus.NewTextLoggerus(loggerName,
			LoggerLevelToLogrusLevel(loggerLevel),
			common.GetRedactorInstance(output),
			enrichWhoField,
			!noColor)
	case LoggerFormatterKindJSON:
		return loggerus.NewJSONLoggerus(loggerName,
			LoggerLevelToLogrusLevel(loggerLevel),
			common.GetRedactorInstance(output))
	default:
		return nil, errors.Errorf("Unexpected logger formatter kind %s", loggerFormatterKindText)
	}
}

func LoggerLevelToLogrusLevel(level logger.Level) logrus.Level {
	switch level {
	case logger.LevelInfo:
		return logrus.InfoLevel
	case logger.LevelWarn:
		return logrus.WarnLevel
	case logger.LevelError:
		return logrus.ErrorLevel
	default:
		return logrus.DebugLevel
	}
}
