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

package platformconfig

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/ghodss/yaml"
)

type Reader struct {
	logger logger.Logger
}

func NewReader(parentLogger logger.Logger) (*Reader, error) {
	return &Reader{
		logger: parentLogger.GetChild("platformConfig"),
	}, nil
}

func (r *Reader) Read(reader io.Reader, configType string, config *Configuration) error {
	configBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return errors.Wrap(err, "Failed to read platform configuration")
	}

	return yaml.Unmarshal(configBytes, config)
}

func (r *Reader) ReadFileOrDefault(configurationPath string) (*Configuration, error) {
	var platformConfiguration Configuration

	// if there's no configuration file, return a default configuration. otherwise try to parse it
	platformConfigurationFile, err := os.Open(configurationPath)
	if err != nil {

		// log whether we're running a default configuration
		r.logger.WarnWith("Platform configuration not found, using defaults", "path", configurationPath)

		return r.GetDefaultConfiguration(), nil
	}

	if err := r.Read(platformConfigurationFile, "yaml", &platformConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to read configuration file")
	}

	return &platformConfiguration, nil
}

func (r *Reader) GetDefaultConfiguration() *Configuration {
	trueValue := true

	return &Configuration{
		WebAdmin: WebServer{
			Enabled:       &trueValue,
			ListenAddress: ":8081",
		},
		HealthCheck: WebServer{
			Enabled:       &trueValue,
			ListenAddress: ":8082",
		},
		Logger: Logger{

			// create an stdout sink and bind everything to it @ debug level
			Sinks: map[string]LoggerSink{
				"stdout": {Kind: "stdout"},
			},

			System: []LoggerSinkBinding{
				{Level: "debug", Sink: "stdout"},
			},

			Functions: []LoggerSinkBinding{
				{Level: "debug", Sink: "stdout"},
			},
		},
	}
}
