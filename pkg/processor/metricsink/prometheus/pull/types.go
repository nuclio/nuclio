/*
Copyright 2023 The Nuclio Authors.

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

package prometheuspull

import (
	"os"

	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor/metricsink"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
)

type Configuration struct {
	metricsink.Configuration
	InstanceName string
}

func NewConfiguration(name string, metricSinkConfiguration *platformconfig.MetricSink) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	newConfiguration.Configuration = *metricsink.NewConfiguration(name, metricSinkConfiguration)

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	if newConfiguration.URL == "" {
		newConfiguration.URL = ":8090"
	}

	envInstanceName := os.Getenv("NUCLIO_FUNCTION_INSTANCE")
	if newConfiguration.InstanceName == "" {
		if envInstanceName == "" {
			newConfiguration.InstanceName = "{{ .Namespace }}-{{ .Name }}"
		} else {
			newConfiguration.InstanceName = envInstanceName
		}
	}

	return &newConfiguration, nil
}
