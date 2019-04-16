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

func (r *Reader) Read(reader io.Reader, configType string, config *Config) error {
	// Performs deep merge of the received and the base config metadata. (received config values doesn't override base metadata values)
	// In order to do that we should create maps from both configs, so we can use mergo.Merge() on their metadata

	var receivedConfigAsMap, baseConfigAsMap map[string]interface{}

	bodyBytes, err := ioutil.ReadAll(reader)

	if err != nil {
		return errors.Wrap(err, "Failed to read configuration file")
	}

	// parse base config to JSON - so its metadata can be merged without override as we want it to
	baseConfigAsJSON, err := json.Marshal(config)
	if err != nil {
		return errors.Wrap(err, "Failed to parse base config to JSON")
	}

	// copy everything from the given config with precedence on everything over the base config
	if err := yaml.Unmarshal(bodyBytes, config); err != nil {
		return errors.Wrap(err, "Failed to write configuration")
	}

	if err = yaml.Unmarshal(bodyBytes, &receivedConfigAsMap); err != nil {
		return errors.Wrap(err, "Failed to parse received config")
	}

	// create a map from the JSON of the base map
	if err = json.Unmarshal(baseConfigAsJSON, &baseConfigAsMap); err != nil {
		return errors.Wrap(err, "Failed to parse base config as JSON to map")
	}

	// merge base config with received config - and make base config values override received config values
	if err = mergo.Merge(&baseConfigAsMap, receivedConfigAsMap); err != nil {
		return errors.Wrap(err, "Failed to merge base config and received config")
	}

	if _, metadataExists := baseConfigAsMap["metadata"]; metadataExists {

		// parse the modified base config map meta to be as JSON, so it can be easily unmarshalled into the config metadata struct
		mergedConfigMetaAsJSON, _ := json.Marshal(baseConfigAsMap["metadata"])
		if err = json.Unmarshal(mergedConfigMetaAsJSON, &config.Meta); err != nil {
			return errors.Wrap(err, "Failed to parse new config from JSON to *Config struct")
		}
	}

	return nil
}
