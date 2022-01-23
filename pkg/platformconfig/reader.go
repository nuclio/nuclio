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

	"github.com/ghodss/yaml"
	"github.com/nuclio/errors"
)

type Reader struct{}

func NewReader() (*Reader, error) {
	return &Reader{}, nil
}

func (r *Reader) Read(reader io.Reader, configType string, config *Config) error {
	configBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return errors.Wrap(err, "Failed to read platform configuration")
	}

	return yaml.Unmarshal(configBytes, config)
}

func (r *Reader) ReadFileOrDefault(configurationPath string) (*Config, error) {
	var platformConfiguration Config

	// if there's no configuration file, return a default configuration. otherwise try to parse it
	platformConfigurationFile, err := os.Open(configurationPath)
	if err != nil {
		return r.GetDefaultConfiguration(), nil
	}

	// close after
	defer platformConfigurationFile.Close() // nolint: errcheck

	if err := r.Read(platformConfigurationFile, "yaml", &platformConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to read configuration file")
	}

	return &platformConfiguration, nil
}

func (r *Reader) GetDefaultConfiguration() *Config {
	trueValue := true
	defaultSinkName := "stdout"

	return &Config{
		WebAdmin: WebServer{
			Enabled:       &trueValue,
			ListenAddress: ":8081",
		},
		HealthCheck: WebServer{
			Enabled:       &trueValue,
			ListenAddress: ":8082",
		},
		Logger: Logger{

			// create an STDOUT sink and bind everything to it @ debug level
			Sinks: map[string]LoggerSink{
				defaultSinkName: {Kind: LoggerSinkKindStdout},
			},

			System: []LoggerSinkBinding{
				{Level: "debug", Sink: defaultSinkName},
			},

			Functions: []LoggerSinkBinding{
				{Level: "debug", Sink: defaultSinkName},
			},
		},
	}
}
