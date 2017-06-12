package runtime

import "github.com/spf13/viper"

type Configuration struct {
	Name        string
	Version     string
	Description string
}

func NewConfiguration(configuration *viper.Viper) *Configuration {
	return &Configuration{
		Name: configuration.GetString("name"),
		Description: configuration.GetString("description"),
		Version: configuration.GetString("version"),
	}
}
