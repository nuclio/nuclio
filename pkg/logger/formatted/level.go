package formatted

import "github.com/sirupsen/logrus"

// Type aliasing for log level
type Level logrus.Level

// Supported log levels
const (
	Error Level = Level(logrus.ErrorLevel)
	Warn  Level = Level(logrus.WarnLevel)
	Info  Level = Level(logrus.InfoLevel)
	Debug Level = Level(logrus.DebugLevel)
)
