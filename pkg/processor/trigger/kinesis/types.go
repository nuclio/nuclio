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

package kinesis

import (
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/mitchellh/mapstructure"
	"github.com/vmware/vmware-go-kcl/clientlibrary/config"
)

type Configuration struct {
	trigger.Configuration
	AccessKeyID     string
	SecretAccessKey string
	RegionName      string
	StreamName      string
	ApplicationName string
	InitialPosition string
	initialPosition config.InitialPositionInStream
}

func NewConfiguration(ID string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	newConfiguration.Configuration = *trigger.NewConfiguration(ID, triggerConfiguration, runtimeConfiguration)

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	var err error
	newConfiguration.initialPosition, err = resolveInitialPosition(newConfiguration.InitialPosition)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create configuration")
	}

	// TODO: validate

	return &newConfiguration, nil
}

func resolveInitialPosition(initialOffset string) (config.InitialPositionInStream, error) {
	if initialOffset == "" {
		return config.LATEST, nil
	}
	if lower := strings.ToLower(initialOffset); lower == "earliest" {
		return config.TRIM_HORIZON, nil
	} else if lower == "latest" {
		return config.LATEST, nil
	} else {
		return 0, errors.Errorf("InitialOffset must be either 'earliest' or 'latest', not '%s'", initialOffset)
	}
}
