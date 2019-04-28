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

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/ghodss/yaml"
	"github.com/imdario/mergo"
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

// Merges codeEntry config with the function config.
// Config populated values won't get override by codeEntry values.
// Enriches config with env vars existing only in codeEntry config
func (r *Reader) Read(reader io.Reader, configType string, config *Config) error {
	var codeEntryConfigMap, configMap map[string]interface{}
	var codeEntryConfig Config

	bodyBytes, err := ioutil.ReadAll(reader)

	if err != nil {
		return errors.Wrap(err, "Failed to read configuration file")
	}

	// load codeEntry config into a Config struct
	if err := yaml.Unmarshal(bodyBytes, &codeEntryConfig); err != nil {
		return errors.Wrap(err, "Failed to write configuration")
	}

	// enrich config with env vars existing only in codeEntry config
	if codeEntryConfig.Spec.Env != nil && config.Spec.Env != nil {
		for _, codeEntryEnvVar := range codeEntryConfig.Spec.Env {
			if !common.EnvInSlice(codeEntryEnvVar, config.Spec.Env) {
				config.Spec.Env = append(config.Spec.Env, codeEntryEnvVar)
			}
		}
	}

	if err = yaml.Unmarshal(bodyBytes, &codeEntryConfigMap); err != nil {
		return errors.Wrap(err, "Failed to parse received config")
	}

	// parse config to JSON - in order to parse it afterwards into a map
	configJSON, err := json.Marshal(config)
	if err != nil {
		return errors.Wrap(err, "Failed to parse config to JSON")
	}

	// create a map from the JSON of the config. we need it as a map so we will be able to use mergo.Merge()
	if err = json.Unmarshal(configJSON, &configMap); err != nil {
		return errors.Wrap(err, "Failed to parse config as JSON to map")
	}

	// merge config with codeEntry config - config populated values won't get overridden by codeEntry config values
	if err = mergo.Merge(&configMap, &codeEntryConfigMap); err != nil {
		return errors.Wrap(err, "Failed to merge config and codeEntry config")
	}

	// parse the modified config map to be as JSON, so it can be easily unmarshalled into the config struct
	mergedConfigJSON, err := json.Marshal(configMap)
	if err != nil {
		return errors.Wrap(err, "Failed to parse new config from from map to JSON")
	}

	// load merged config into the function config
	if err = json.Unmarshal(mergedConfigJSON, &config); err != nil {
		return errors.Wrap(err, "Failed to parse new config from JSON to *Config struct")
	}

	return nil
}
