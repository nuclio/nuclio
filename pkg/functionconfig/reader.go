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

	"github.com/nuclio/nuclio-sdk"
	"github.com/spf13/viper"
)

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

func (r *Reader) Read(reader io.Reader, configType string, config *Config) error {
	r.functionConfigViper.SetConfigType(configType)

	if err := r.functionConfigViper.ReadConfig(reader); err != nil {
		return errors.Wrap(err, "Failed to read configuration file")
	}

	// unmarshal to config
	return r.functionConfigViper.Unmarshal(config)
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
