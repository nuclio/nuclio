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
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
)

// CreateSystemLoggers returns the system loggers
func CreateSystemLogger(name string, platformConfiguration *platformconfig.Config) (logger.Logger, error) {

	// get system loggers
	systemLoggerSinksByName, err := platformConfiguration.GetSystemLoggerSinks()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get system logger sinks")
	}

	return createLoggers(name, systemLoggerSinksByName)
}

// returns the processor logger and the function logger. For now, they are one of the same
func CreateFunctionLogger(name string,
	functionConfiguration *functionconfig.Config,
	platformConfiguration *platformconfig.Config) (logger.Logger, error) {

	// get system loggers
	functionLoggerSinksByName, err := platformConfiguration.GetFunctionLoggerSinks(functionConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get system logger sinks")
	}

	return createLoggers(name, functionLoggerSinksByName)
}

// returns the processor logger and the function logger. For now, they are one of the same
func createLoggers(name string, loggerSinksWithLevel map[string]platformconfig.LoggerSinkWithLevel) (logger.Logger, error) {
	var loggers []logger.Logger
	var loggerInstance logger.Logger
	var err error

	// get system logger sinks
	for _, loggerSinkConfiguration := range loggerSinksWithLevel {
		var loggerInstance logger.Logger

		loggerInstance, err = RegistrySingleton.NewLoggerSink(loggerSinkConfiguration.Sink.Kind,
			name,
			&loggerSinkConfiguration)

		if err != nil {
			return nil, errors.Wrap(err, "Failed to create logger")
		}

		// add logger to system loggers
		loggers = append(loggers, loggerInstance)
	}

	// if there's more than one logger, create a mux logger (as it does carry _some_ overhead over a single logger)
	if len(loggers) > 1 {

		// create system logger
		loggerInstance, err = nucliozap.NewMuxLogger(loggers...)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to created system mux logger")
		}

	} else {
		if len(loggers) == 0 {
			return nil, errors.New("Must configure at least one logger")
		}

		loggerInstance = loggers[0]
	}

	return loggerInstance, nil
}
