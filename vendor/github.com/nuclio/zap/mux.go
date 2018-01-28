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

package nucliozap

import "github.com/nuclio/logger"

// a logger that multiplexes logs towards multiple loggers
type MuxLogger struct {
	loggers []logger.Logger
}

func NewMuxLogger(loggers ...logger.Logger) (*MuxLogger, error) {
	return &MuxLogger{loggers: loggers}, nil
}

func (ml *MuxLogger) SetLoggers(loggers ...logger.Logger) {
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

func (ml *MuxLogger) GetChild(name string) logger.Logger {
	return ml
}
