package platform

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/nuclio/nuclio-sdk"
)

type Function interface {

	// Initialize instructs the function to load the fields specified by "fields". Some function implementations
	// are lazy-load - this ensures that the fields are populated properly. if "fields" is nil, all fields
	// are loaded
	Initialize([]string) error

	// GetConfig will return the configuration of the function
	GetConfig() *functionconfig.Config

	// GetState returns the state of the function
	GetState() string

	// GetClusterIP gets the IP of the cluster hosting the function
	GetClusterIP() string
}

type AbstractFunction struct {
	Logger nuclio.Logger
	Config functionconfig.Config
}

func NewAbstractFunction(parentLogger nuclio.Logger, config *functionconfig.Config) (*AbstractFunction, error) {
	return &AbstractFunction{
		Logger: parentLogger.GetChild("function"),
		Config: *config,
	}, nil
}

func (af *AbstractFunction) GetConfig() *functionconfig.Config {
	return &af.Config
}