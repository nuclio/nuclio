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

package trigger

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
)

type Configuration struct {
	functionconfig.Trigger

	// the runtime configuration, for reference
	RuntimeConfiguration *runtime.Configuration

	// a unique trigger ID
	ID string
}

func NewConfiguration(ID string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) *Configuration {

	configuration := &Configuration{
		Trigger:              *triggerConfiguration,
		RuntimeConfiguration: runtimeConfiguration,
		ID:                   ID,
	}

	// set defaults
	if configuration.MaxWorkers == 0 {
		configuration.MaxWorkers = 1
	}

	return configuration
}

type Statistics struct {
	EventsHandleSuccessTotal uint64
	EventsHandleFailureTotal uint64
}

func (s *Statistics) DiffFrom(prev *Statistics) Statistics {
	return Statistics{
		EventsHandleSuccessTotal: s.EventsHandleSuccessTotal - prev.EventsHandleSuccessTotal,
		EventsHandleFailureTotal: s.EventsHandleFailureTotal - prev.EventsHandleFailureTotal,
	}
}
