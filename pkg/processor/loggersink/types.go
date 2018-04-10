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
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/logger"
)

type Configuration struct {
	platformconfig.LoggerSinkWithLevel
	Name  string
	Level logger.Level
}

func NewConfiguration(name string, loggerSinkConfiguration *platformconfig.LoggerSinkWithLevel) *Configuration {
	var level logger.Level

	switch loggerSinkConfiguration.Level {
	case "info":
		level = logger.LevelInfo
	case "warn":
		level = logger.LevelWarn
	case "error":
		level = logger.LevelError
	default:
		level = logger.LevelDebug
	}

	configuration := &Configuration{
		LoggerSinkWithLevel: *loggerSinkConfiguration,
		Name:                name,
		Level:               level,
	}

	return configuration
}
