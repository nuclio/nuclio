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

package loggersink

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
)

// returns the processor logger and the function logger. For now, they are one of the same
func CreateLoggers(name string, platformConfiguration *platformconfig.Configuration) (logger.Logger, logger.Logger, error) {
	var systemLogger logger.Logger

	// holds system loggers
	var systemLoggers []logger.Logger

	// get system loggers
	systemLoggerSinksByName, err := platformConfiguration.GetSystemLoggerSinks()
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to get system logger sinks")
	}

	// get system logger sinks
	for _, loggerSinkConfiguration := range systemLoggerSinksByName {
		var loggerInstance logger.Logger

		loggerInstance, err = RegistrySingleton.NewLoggerSink(loggerSinkConfiguration.Sink.Kind,
			name,
			&loggerSinkConfiguration)

		if err != nil {
			return nil, nil, errors.Wrap(err, "Failed to create logger")
		}

		// add logger to system loggers
		systemLoggers = append(systemLoggers, loggerInstance)
	}

	// if there's more than one logger, create a mux logger (as it does carry _some_ overhead over a single logger)
	if len(systemLoggers) > 1 {

		// create system logger
		systemLogger, err = nucliozap.NewMuxLogger(systemLoggers...)
		if err != nil {
			return nil, nil, errors.Wrap(err, "Failed to created system mux logger")
		}

	} else {
		systemLogger = systemLoggers[0]
	}

	return systemLogger, systemLogger, nil
}
