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

package config

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/viper"
)

var (
	// Sections is the list of section in configuration
	Sections = []string{
		"event_sources",
		"function",
		"logger",
		"web_admin",
	}
)

// ReadProcessorConfiguration reader processor configuration from file
func ReadProcessorConfiguration(configurationPath string) (map[string]*viper.Viper, error) {

	// if no configuration file passed use defaults all around
	if configurationPath == "" {
		return nil, nil
	}

	root := viper.New()
	root.SetConfigFile(configurationPath)

	// read the root configuration file
	if err := root.ReadInConfig(); err != nil {
		return nil, err
	}

	config := map[string]*viper.Viper{"root": root}

	// get the directory of the root configuration file, we'll need it since all section
	// configuration files are relative to that
	rootConfigurationDir := filepath.Dir(configurationPath)

	// read the configuration file sections, which may be in separate configuration files or inline
	for _, sectionName := range Sections {

		// try to get <section name>.config_path (e.g. function.config_path)
		sectionConfigPath := root.GetString(fmt.Sprintf("%s.config_path", sectionName))

		// if it exists, create a viper and read it
		if sectionConfigPath != "" {
			config[sectionName] = viper.New()
			sectionFilePath := filepath.Join(rootConfigurationDir, sectionConfigPath)
			config[sectionName].SetConfigFile(sectionFilePath)

			// do the read
			if err := config[sectionName].ReadInConfig(); err != nil {
				return nil, err
			}
		} else {
			// the section is a sub of the root
			config[sectionName] = config["root"].Sub(sectionName)
		}
	}

	return config, nil
}
