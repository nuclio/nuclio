package databinding

import "github.com/nuclio/nuclio/pkg/functionconfig"

type Configuration struct {
	functionconfig.DataBinding
	ID string
}

func NewConfiguration(ID string, databindingConfiguration *functionconfig.DataBinding) *Configuration {
	configuration := &Configuration{
		DataBinding: *databindingConfiguration,
		ID:          ID,
	}

	return configuration
}
