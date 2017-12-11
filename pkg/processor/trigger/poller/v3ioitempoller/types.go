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

package v3ioitempoller

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger/poller"

	"github.com/mitchellh/mapstructure"
)

type Configuration struct {
	poller.Configuration
	Restart        bool
	URL            string
	ContainerID    int
	ContainerAlias string
	Paths          []string
	Attributes     []string
	Queries        []string
	Suffixes       []string
	Incremental    bool
	ShardID        int
	TotalShards    int
}

func NewConfiguration(ID string, triggerConfiguration *functionconfig.Trigger) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	pollerConfiguration, err := poller.NewConfiguration(ID, triggerConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read poller configuration")
	}

	newConfiguration.Configuration = *pollerConfiguration

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	// TODO: validate

	return &newConfiguration, nil
}
