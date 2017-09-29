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

package runtime

import (
	"github.com/nuclio/nuclio-sdk"
)

type Runtime interface {
	ProcessEvent(event nuclio.Event, functionLogger nuclio.Logger) (interface{}, error)

	GetFunctionLogger() nuclio.Logger

	GetStatistics() *Statistics
}

type AbstractRuntime struct {
	Logger         nuclio.Logger
	FunctionLogger nuclio.Logger
	Context        *nuclio.Context
	Statistics     Statistics
}

func NewAbstractRuntime(logger nuclio.Logger,
	configuration *Configuration) (*AbstractRuntime, error) {

	context, err := newContext(configuration.FunctionLogger, configuration)
	if err != nil {
		return nil, err
	}

	return &AbstractRuntime{
		Logger:         logger,
		FunctionLogger: configuration.FunctionLogger,
		Context:        context,
	}, nil
}

func (ar *AbstractRuntime) GetFunctionLogger() nuclio.Logger {
	return ar.FunctionLogger
}

func (ar *AbstractRuntime) GetStatistics() *Statistics {
	return &ar.Statistics
}
