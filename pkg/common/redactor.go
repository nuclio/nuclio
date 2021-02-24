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

package common

import (
	"io"

	"github.com/nuclio/logger"
	"github.com/nuclio/loggerus"
)

var defaultValueRedactions = []string{
	"password",
}

func GetRedactorInstance(output io.Writer) *loggerus.Redactor {
	redactorInstance := loggerus.NewRedactor(output)
	redactorInstance.AddValueRedactions(defaultValueRedactions)
	return redactorInstance
}

func SetLoggerRedactionMode(loggerInstance logger.Logger, enabled bool) {
	for _, loggerFromLoggerInstance := range GetLoggersFromInstance(loggerInstance) {
		ApplyRedactorChange(loggerFromLoggerInstance, func(redactor *loggerus.Redactor) {
			if enabled {
				redactor.Enable()
			} else {
				redactor.Disable()
			}
		})
	}
}

func GetLoggersFromInstance(loggerInstance logger.Logger) []logger.Logger {
	muxLogger, loggerIsMuxLogger := loggerInstance.(*loggerus.MuxLogger)
	if loggerIsMuxLogger {
		return muxLogger.GetLoggers()
	}

	return []logger.Logger{loggerInstance}
}

func ApplyRedactorChange(loggerInstance logger.Logger, callback func(*loggerus.Redactor)) {
	redactingLogger, loggerIsRedactingLogger := loggerInstance.(loggerus.RedactingLogger)
	if loggerIsRedactingLogger && redactingLogger.GetRedactor() != nil {
		callback(redactingLogger.GetRedactor())
	}
}
