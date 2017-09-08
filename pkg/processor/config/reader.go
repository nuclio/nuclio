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
