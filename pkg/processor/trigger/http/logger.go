package http

import (
	"strings"

	"github.com/nuclio/logger"
)

type FastHTTPLogger struct {
	logger logger.Logger
}

func NewFastHTTPLogger(parentLogger logger.Logger) FastHTTPLogger {
	return FastHTTPLogger{parentLogger.GetChild("fasthttp")}
}

func (s FastHTTPLogger) Printf(format string, args ...interface{}) {
	s.output(format, args...)
}

func (s FastHTTPLogger) output(format string, args ...interface{}) {
	s.logger.Debug("FastHTTP: "+strings.TrimSuffix(format, "\n"), args...)
}
