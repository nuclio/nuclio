package event_source

import "github.com/spf13/viper"

type Configuration struct {
	ID string
}

func NewConfiguration(configuration *viper.Viper) *Configuration {
	return &Configuration{
		ID: configuration.GetString("ID"),
	}
}
