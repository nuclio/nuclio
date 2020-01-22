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
	s.output(format, v...)
}

func (s SaramaLogger) Printf(format string, v ...interface{}) {
	s.output(format, v...)
}

func (s SaramaLogger) Println(v ...interface{}) {
	format := strings.Repeat("%v", len(v))
	s.output(format, v...)
}

func (s SaramaLogger) output(format string, v ...interface{}) {
	s.logger.Debug("Sarama: "+strings.TrimSuffix(format, "\n"), v...)
}
