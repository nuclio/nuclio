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
	"io"
	"io/ioutil"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/ghodss/yaml"
	"github.com/nuclio/logger"
)

type Reader struct {
	logger logger.Logger
}

func NewReader(parentLogger logger.Logger) (*Reader, error) {
	return &Reader{
		logger: parentLogger.GetChild("reader"),
	}, nil
}

func (r *Reader) Read(reader io.Reader, configType string, config *Config) error {
	bodyBytes, err := ioutil.ReadAll(reader)

	if err != nil {
		return errors.Wrap(err, "Failed to read configuration file")
	}

	if err = r.unmarshalYamlWithoutOverridingFields(bodyBytes, config); err != nil {
		return errors.Wrap(err, "Failed to write configuration")
	}

	return nil
}

func (r *Reader) unmarshalYamlWithoutOverridingFields(bodyBytes []byte, config *Config) error {
	// yaml.Unamrshal overrides fields, so in order to update config fields from a configuration yaml
	// without overriding existing fields we:
	// 1. Create a temp config with the fields of the given yaml file
	// 2. Parse the current config into a yaml file
	// 3. Unmarshal the generated current config yaml file onto the temp config - to override fields that exist on both.
	// 4. Set the current config pointer to point to the tmpConfig

	var tmpConfig *Config

	if err := yaml.Unmarshal(bodyBytes, tmpConfig); err != nil {
		return err
	}

	// parse the current config to yaml
	currentConfigAsJson, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	// load the current config onto the tmpConfig - making it override the fields it has
	if err = yaml.Unmarshal(currentConfigAsJson,tmpConfig); err != nil {
		return err
	}

	config = tmpConfig

	return nil
}
