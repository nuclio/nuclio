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
	"encoding/json"
	"io"
	"io/ioutil"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/ghodss/yaml"
	"github.com/imdario/mergo"
)

type Reader struct {
	logger logger.Logger
}

func NewReader(parentLogger logger.Logger) (*Reader, error) {
	return &Reader{
		logger: parentLogger.GetChild("reader"),
	}, nil
}


// Merges codeEntry config with base function config.
// Base config populated values won't get override by codeEntry values.
// The only exception is that there will be a union of the environment variables, with precedence to base config.
func (r *Reader) Read(reader io.Reader, configType string, config *Config) error {
	var codeEntryConfigAsMap, baseConfigAsMap map[string]interface{}
	var codeEntryConfig Config

	bodyBytes, err := ioutil.ReadAll(reader)

	if err != nil {
		return errors.Wrap(err, "Failed to read configuration file")
	}

	// load codeEntry configuration into a Config struct
	if err := yaml.Unmarshal(bodyBytes, &codeEntryConfig); err != nil {
		return errors.Wrap(err, "Failed to write configuration")
	}

	// merge env vars, with precedence to base config env vars
	if codeEntryConfig.Spec.Env != nil && config.Spec.Env != nil {
		for _, codeEntryEnvVar := range codeEntryConfig.Spec.Env {
			existsInBaseEnv := false
			for _, baseEnvVar := range config.Spec.Env {
				if baseEnvVar.Name == codeEntryEnvVar.Name {
					existsInBaseEnv = true
					break
				}
			}
			if !existsInBaseEnv {
				config.Spec.Env = append(config.Spec.Env, codeEntryEnvVar)
			}
		}
	}

	if err = yaml.Unmarshal(bodyBytes, &codeEntryConfigAsMap); err != nil {
		return errors.Wrap(err, "Failed to parse received config")
	}

	// parse base config to JSON - in order to parse it afterwards into a map
	baseConfigAsJSON, err := json.Marshal(config)
	if err != nil {
		return errors.Wrap(err, "Failed to parse base config to JSON")
	}

	// create a map from the JSON of the base map. we need it as a map so we will be able to use mergo.Merge()
	if err = json.Unmarshal(baseConfigAsJSON, &baseConfigAsMap); err != nil {
		return errors.Wrap(err, "Failed to parse base config as JSON to map")
	}

	// merge base config with received config - and make base config values override codeEntry config values
	if err = mergo.Merge(&baseConfigAsMap, &codeEntryConfigAsMap); err != nil {
		return errors.Wrap(err, "Failed to merge base config and received config")
	}

	// parse the modified base config map to be as JSON, so it can be easily unmarshalled into the config struct
	mergedConfigAsJSON, err := json.Marshal(baseConfigAsMap)
	if err != nil {
		return errors.Wrap(err, "Failed to parse new config from from map to JSON")
	}

	// load merged config into the function config
	if err = json.Unmarshal(mergedConfigAsJSON, &config); err != nil {
		return errors.Wrap(err, "Failed to parse new config from JSON to *Config struct")
	}

	return nil
}
