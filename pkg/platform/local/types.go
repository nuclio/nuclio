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

package local

import (
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
)

type functionPlatformConfiguration struct {
	Network       string
	RestartPolicy *dockerclient.RestartPolicy
	MountMode     FunctionMountMode

	// Deprecated. Will be removed in the next minor (1.6.x) version release
	ProcessorMountMode FunctionMountMode
}

func newFunctionPlatformConfiguration(functionConfig *functionconfig.Config) (*functionPlatformConfiguration, error) {
	newConfiguration := functionPlatformConfiguration{}

	// parse attributes
	if err := mapstructure.Decode(functionConfig.Spec.Platform.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	// if mount mode field is not set, shove deprecated field value to it, it might contain a value
	if newConfiguration.MountMode == "" {
		newConfiguration.MountMode = newConfiguration.ProcessorMountMode
	}

	// anyway, empty out value
	newConfiguration.ProcessorMountMode = ""

	return &newConfiguration, nil
}

type FunctionMountMode string

const (

	// use `/tmp` host path directory and bind mount
	FunctionMountModeBind FunctionMountMode = "bind"

	// create a docker volume and mount it
	FunctionMountModeVolume FunctionMountMode = "volume"
)
