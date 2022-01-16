package common

import (
	"io"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
)

var defaultValueRedactions = []string{
	"password",
	"cookie",
	"x-v3io-session-key",
	"sessionKey",
	"token",
	"clientSecret",
	"secret",
	"accessKey",
	"accessCertificate",
	"caCert",
}

func GetRedactorInstance(output io.Writer) *nucliozap.Redactor {
	redactorInstance := nucliozap.NewRedactor(output)
	redactorInstance.AddValueRedactions(defaultValueRedactions)
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