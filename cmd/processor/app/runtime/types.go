package runtime

import (
	"github.com/nuclio/nuclio/pkg/util/common"
	"github.com/spf13/viper"
)

// TODO: this should be a whole thing. for now this should do
type DataBinding struct {
	Class string
	URL   string
}

type Configuration struct {
	Name         string
	Version      string
	Description  string
	DataBindings []DataBinding
}

func NewConfiguration(configuration *viper.Viper) (*Configuration, error) {

	newConfiguration := &Configuration{
		Name:        configuration.GetString("name"),
		Description: configuration.GetString("description"),
		Version:     configuration.GetString("version"),
	}

	// read data bindings
	dataBindings := common.GetObjectSlice(configuration, "data_bindings")
	for _, dataBinding := range dataBindings {
		newDataBinding := DataBinding{
			Class: dataBinding["class"].(string),
			URL:   dataBinding["url"].(string),
		}

		newConfiguration.DataBindings = append(newConfiguration.DataBindings, newDataBinding)
	}

	return newConfiguration, nil
}
