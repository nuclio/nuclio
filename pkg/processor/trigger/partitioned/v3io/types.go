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

package v3io

import (
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger/partitioned"

	"github.com/mitchellh/mapstructure"
)

type seekToType string

const (
	seekToTypeLatest  seekToType = "latest"
	seekToTypeEarlist            = "earliest"
)

type Configuration struct {
	partitioned.Configuration
	Partitions          []int
	NumContainerWorkers int
	SeekTo              string
	ReadBatchSize       int
	PollingIntervalMs   int
}

func NewConfiguration(ID string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	newConfiguration.Configuration = *partitioned.NewConfiguration(ID, triggerConfiguration, runtimeConfiguration)

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	if newConfiguration.NumContainerWorkers == 0 {
		newConfiguration.NumContainerWorkers = len(newConfiguration.Partitions)/2 + 1
	}

	if newConfiguration.ReadBatchSize == 0 {
		newConfiguration.ReadBatchSize = 64
	}

	if newConfiguration.PollingIntervalMs == 0 {
		newConfiguration.PollingIntervalMs = 500
	}

	if newConfiguration.SeekTo == "" {
		newConfiguration.SeekTo = string(seekToTypeLatest)
	}

	if !strings.HasPrefix(newConfiguration.URL, "http") {
		newConfiguration.URL = "http://" + newConfiguration.URL
	}

	return &newConfiguration, nil
}
