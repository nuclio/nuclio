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
	"io"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
)

func GetRedactorInstance(output io.Writer) *nucliozap.Redactor {
	redactorInstance := nucliozap.NewRedactor(output)

	// TODO: Add redaction values (e.g.: "password") once json formatter is fully supported
	// Note: there's an issue with redact values when they are fully escaped
	return redactorInstance
}

func SetLoggerRedactionMode(loggerInstance logger.Logger, enabled bool) {
	for _, loggerInstance := range GetLoggersFromInstance(loggerInstance) {
		ApplyRedactorChange(loggerInstance, func(redactor *nucliozap.Redactor) {
			if enabled {
				redactor.Enable()
			} else {
				redactor.Disable()
			}
		})
	}
}

func GetLoggersFromInstance(loggerInstance logger.Logger) []logger.Logger {
	muxLogger, loggerIsMuxLogger := loggerInstance.(*nucliozap.MuxLogger)
	if loggerIsMuxLogger {
		return muxLogger.GetLoggers()
	}

	return []logger.Logger{loggerInstance}
}

func ApplyRedactorChange(loggerInstance logger.Logger, callback func(*nucliozap.Redactor)) {
	redactingLogger, loggerIsRedactingLogger := loggerInstance.(nucliozap.RedactingLogger)
	if loggerIsRedactingLogger && redactingLogger.GetRedactor() != nil {
		callback(redactingLogger.GetRedactor())
	}
}
