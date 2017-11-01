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

package functionconfig

import (
	"fmt"
	"io"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/nuclio-sdk"
	"github.com/spf13/viper"
)

type field struct {
	name  string
	value interface{}
}

type Reader struct {
	logger              nuclio.Logger
	functionConfigViper *viper.Viper
}

func NewReader(parentLogger nuclio.Logger) (*Reader, error) {
	return &Reader{
		logger:              parentLogger.GetChild("reader"),
		functionConfigViper: viper.New(),
	}, nil
}

func (r *Reader) Read(reader io.Reader, configType string) error {
	r.functionConfigViper.SetConfigType(configType)

	if err := r.functionConfigViper.ReadConfig(reader); err != nil {
		return errors.Wrap(err, "Failed to read configuration file")
	}

	// check if this is k8s formatting
	if r.functionConfigViper.IsSet("apiVersion") {
		return errors.New("Kubernetes specfile format not supported yet")
	}

	return nil
}

func (r *Reader) ToDeployOptions(deployOptions *platform.DeployOptions) error {

	// unmarshall to a deploy options structure
	if err := r.functionConfigViper.Unmarshal(deployOptions); err != nil {
		return errors.Wrap(err, "Failed to unmarshal to deploy options")
	}

	// read stuff that isn't naturally aligned
	for _, deployField := range []field{
		{"name", &deployOptions.CommonOptions.Identifier},
		{"namespace", &deployOptions.CommonOptions.Namespace},
		{"runtime", &deployOptions.Build.Runtime},
		{"handler", &deployOptions.Build.Handler},
	} {
		if err := r.readFieldIfSet(r.functionConfigViper, deployField.name, deployField.value); err != nil {
			return errors.Wrap(err, "Failed to read field")
		}
	}

	return nil
}

func (r *Reader) ToBuildOptions(buildOptions *platform.BuildOptions) error {

	functionConfigBuildViper := r.functionConfigViper.Sub("build")
	if functionConfigBuildViper == nil {
		r.logger.DebugWith("No 'build' key found in function configuration")

		return nil
	}

	// unmarshall to a build options structure
	if err := functionConfigBuildViper.Unmarshal(buildOptions); err != nil {
		return errors.Wrap(err, "Failed to unmarshal to build options")
	}

	// read stuff that isn't naturally aligned
	for _, deployField := range []field{
		{"runtime", &buildOptions.Runtime},
		{"handler", &buildOptions.Handler},
	} {
		if err := r.readFieldIfSet(r.functionConfigViper, deployField.name, deployField.value); err != nil {
			return errors.Wrap(err, "Failed to read field")
		}
	}

	return nil
}

func (r *Reader) readFieldIfSet(inputViper *viper.Viper, fieldName string, fieldValue interface{}) error {
	if inputViper.IsSet(fieldName) {
		switch fieldValue.(type) {
		case *string:
			*fieldValue.(*string) = inputViper.GetString(fieldName)
		case *int:
			*fieldValue.(*int) = inputViper.GetInt(fieldName)
		default:
			return fmt.Errorf("Skipped field %s - unsupported type", fieldName)
		}
	}

	return nil
}
