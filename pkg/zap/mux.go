package nucliozap

import "github.com/nuclio/nuclio-sdk"

// a logger that multiplexes logs towards multiple loggers
type MuxLogger struct {
	loggers []nuclio.Logger
}

func NewMuxLogger(loggers ...nuclio.Logger) (*MuxLogger, error) {
	return &MuxLogger{loggers: loggers}, nil
}

func (ml *MuxLogger) SetLoggers(loggers ...nuclio.Logger) {
	ml.loggers = loggers
}

func (ml *MuxLogger) Error(format interface{}, vars ...interface{}) {
	for _, logger := range ml.loggers {
		logger.Error(format, vars...)
	}
}

func (ml *MuxLogger) Warn(format interface{}, vars ...interface{}) {
	for _, logger := range ml.loggers {
		logger.Warn(format, vars...)
	}
}

func (ml *MuxLogger) Info(format interface{}, vars ...interface{}) {
	for _, logger := range ml.loggers {
		logger.Info(format, vars...)
	}
}

func (ml *MuxLogger) Debug(format interface{}, vars ...interface{}) {
	for _, logger := range ml.loggers {
		logger.Debug(format, vars...)
	}
}

func (ml *MuxLogger) ErrorWith(format interface{}, vars ...interface{}) {
	for _, logger := range ml.loggers {
		logger.ErrorWith(format, vars...)
	}
}

func (ml *MuxLogger) WarnWith(format interface{}, vars ...interface{}) {
	for _, logger := range ml.loggers {
		logger.WarnWith(format, vars...)
	}
}

func (ml *MuxLogger) InfoWith(format interface{}, vars ...interface{}) {
	for _, logger := range ml.loggers {
		logger.InfoWith(format, vars...)
	}
}

func (ml *MuxLogger) DebugWith(format interface{}, vars ...interface{}) {
	for _, logger := range ml.loggers {
		logger.DebugWith(format, vars...)
	}
}

func (ml *MuxLogger) Flush() {
}

func (ml *MuxLogger) GetChild(name string) interface{} {
	return ml
}
