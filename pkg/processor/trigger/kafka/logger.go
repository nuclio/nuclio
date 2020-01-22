package kafka

import (
	"strings"

	"github.com/nuclio/logger"
)

type SaramaLogger struct {
	logger logger.Logger
}

func NewSaramaLogger(parentLogger logger.Logger) SaramaLogger {
	return SaramaLogger{parentLogger.GetChild("sarama")}
}

func (s SaramaLogger) Print(v ...interface{}) {
	format := strings.Repeat("%v", len(v))
	s.logger.Debug(strings.TrimSuffix(format, "\n"), v...)
}

func (s SaramaLogger) Printf(format string, v ...interface{}) {
	s.logger.Debug(strings.TrimSuffix(format, "\n"), v...)
}

func (s SaramaLogger) Println(v ...interface{}) {
	format := strings.Repeat("%v", len(v))
	s.logger.Debug(strings.TrimSuffix(format, "\n"), v...)
}
