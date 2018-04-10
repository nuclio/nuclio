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

package stdout

import (
	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor/loggersink"
)

type Configuration struct {
	loggersink.Configuration
	MaxBatchSize            int
	MaxBatchIntervalSeconds int
	InstrumentationKey      string
}

func NewConfiguration(name string, loggerSinkConfiguration *platformconfig.LoggerSinkWithLevel) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	newConfiguration.Configuration = *loggersink.NewConfiguration(name, loggerSinkConfiguration)

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Sink.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	if newConfiguration.MaxBatchSize == 0 {
		newConfiguration.MaxBatchSize = 1024
	}

	if newConfiguration.MaxBatchIntervalSeconds == 0 {
		newConfiguration.MaxBatchIntervalSeconds = 3
	}

	if newConfiguration.InstrumentationKey == "" {
		return nil, errors.New("InstrumentationKey is required for Application Insights logger sink")
	}

	return &newConfiguration, nil
}
