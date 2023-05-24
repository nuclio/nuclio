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
	"os"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/imdario/mergo"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"sigs.k8s.io/yaml"
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
// CodeEntry config will get overridden by config values.
// Enriches config with env vars existing only in codeEntry config
func (r *Reader) Read(reader io.Reader, configType string, config *Config) error {
	var codeEntryConfigAsMap, configAsMap map[string]interface{}
	var codeEntryConfig Config

	bodyBytes, err := io.ReadAll(reader)

	if err != nil {
		return errors.Wrap(err, "Failed to read configuration file")
	}

	// load codeEntry config into a Config struct
	if err := yaml.Unmarshal(bodyBytes, &codeEntryConfig); err != nil {
		return errors.Wrap(err, "Failed to write configuration")
	}

	err = r.validateConfigurationFileFunctionConfig(&codeEntryConfig)
	if err != nil {
		return errors.Wrap(err, "configuration file invalid")
	}

	// normalizing the received config to the JSON values of the function config Go struct
	codeEntryConfigAsJSON, err := json.Marshal(codeEntryConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to parse received config to JSON")
	}

	if err = yaml.Unmarshal(codeEntryConfigAsJSON, &codeEntryConfigAsMap); err != nil {
		return errors.Wrap(err, "Failed to parse received config")
	}

	// parse config to JSON - in order to parse it afterwards into a map
	configAsJSON, err := json.Marshal(config)
	if err != nil {
		return errors.Wrap(err, "Failed to parse config to JSON")
	}

	// create a map from the JSON of the config. we need it as a map so we will be able to use mergo.Merge()
	if err = json.Unmarshal(configAsJSON, &configAsMap); err != nil {
		return errors.Wrap(err, "Failed to parse config as JSON to map")
	}

	// merge config with codeEntry config - config populated values will take precedence
	if err = mergo.Merge(&configAsMap, &codeEntryConfigAsMap); err != nil {
		return errors.Wrap(err, "Failed to merge config and codeEntry config")
	}

	// parse the modified configAsMap to be as JSON, so it can be easily unmarshalled into the config struct
	mergedConfigAsJSON, err := json.Marshal(configAsMap)
	if err != nil {
		return errors.Wrap(err, "Failed to parse new config from from map to JSON")
	}

	// load merged config into the function config
	if err = json.Unmarshal(mergedConfigAsJSON, &config); err != nil {
		return errors.Wrap(err, "Failed to parse new config from JSON to *Config struct")
	}

	r.enrichPostMergeConfig(config, &codeEntryConfig)

	return nil
}

func (r *Reader) ReadFunctionConfigFile(functionConfigPath string, config *Config) error {

	// read the file once for logging
	functionConfigContents, err := os.ReadFile(functionConfigPath)
	if err != nil {
		return errors.Wrap(err, "Failed to read function configuration file")
	}

	// log
	r.logger.DebugWith("Read function configuration file", "contents", string(functionConfigContents))

	functionConfigFile, err := os.Open(functionConfigPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to open function configuration file: %s", functionConfigPath)
	}

	defer functionConfigFile.Close() // nolint: errcheck

	// read the configuration
	if err := r.Read(functionConfigFile,
		"yaml",
		config); err != nil {

		return errors.Wrap(err, "Failed to read function configuration file")
	}

	return nil
}

func (r *Reader) enrichPostMergeConfig(config, codeEntryConfig *Config) {

	// enrich config with env vars existing only in codeEntry config
	if codeEntryConfig.Spec.Env != nil && config.Spec.Env != nil {
		for _, codeEntryEnvVar := range codeEntryConfig.Spec.Env {
			if !common.EnvInSlice(codeEntryEnvVar, config.Spec.Env) {
				config.Spec.Env = append(config.Spec.Env, codeEntryEnvVar)
			}
		}
	}

	// If one of the configs had the default http trigger and the other had a regular trigger
	// then there would be 2 triggers which isn't valid, in this case we can delete the default trigger.
	// If both configs had regular triggers and not the default, the validation later will fail as expected.
	if len(GetTriggersByKind(config.Spec.Triggers, "http")) > 1 {
		defaultHTTPTrigger := GetDefaultHTTPTrigger()
		delete(config.Spec.Triggers, defaultHTTPTrigger.Name)
	}
}

// There is already validation of the function config pre merge, and validation post merge.
// This validation function is for validation during the merge itself which is mainly to convey to the user
// about validity of the configuration file itself, and therefore help the user understand where his problem lies.
func (r *Reader) validateConfigurationFileFunctionConfig(codeEntryConfig *Config) error {
	if len(GetTriggersByKind(codeEntryConfig.Spec.Triggers, "http")) > 1 {
		return errors.New("FunctionConfig from configuration file cannot have more than 1 http trigger")
	}

	// TODO: decide if we want to add more "mid merge" validations.
	return nil
}
