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

package prometheuspush

import (
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor/metricsink"

	"github.com/mitchellh/mapstructure"
)

type Configuration struct {
	metricsink.Configuration
	Interval       string
	JobName        string
	InstanceName   string
	parsedInterval time.Duration
}

func NewConfiguration(name string, metricSinkConfiguration *platformconfig.MetricSink) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	newConfiguration.Configuration = *metricsink.NewConfiguration(name, metricSinkConfiguration)

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	// try to parse the interval
	var err error
	newConfiguration.parsedInterval, err = time.ParseDuration(newConfiguration.Interval)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse interval")
	}

	// verify job name passed
	if newConfiguration.JobName == "" {
		return nil, fmt.Errorf("Job name is required for metric sink %s", name)
	}

	// verify instance name passed
	if newConfiguration.InstanceName == "" {
		return nil, fmt.Errorf("Instance name is required for metric sink %s", name)
	}

	return &newConfiguration, nil
}
