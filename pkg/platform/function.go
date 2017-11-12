package platform

import (
	"strconv"

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

	// GetInvokeURL returns the URL on which the function can be invoked
	GetInvokeURL(InvokeViaType) (string, error)

	// GetReplicas returns the current # of replicas and the configured # of replicas
	GetReplicas() (int, int)

	// GetIngresses returns all ingresses for this function
	GetIngresses() map[string]functionconfig.Ingress

	// GetVersion returns a string representing the version
	GetVersion() string
}

type AbstractFunction struct {
	Logger   nuclio.Logger
	Config   functionconfig.Config
	Platform Platform
}

func NewAbstractFunction(parentLogger nuclio.Logger,
	parentPlatform Platform,
	config *functionconfig.Config) (*AbstractFunction, error) {

	return &AbstractFunction{
		Logger:   parentLogger.GetChild("function"),
		Config:   *config,
		Platform: parentPlatform,
	}, nil
}

func (af *AbstractFunction) GetConfig() *functionconfig.Config {
	return &af.Config
}

// GetIngresses returns all ingresses for this function
func (af *AbstractFunction) GetIngresses() map[string]functionconfig.Ingress {
	return functionconfig.GetIngressesFromTriggers(af.Config.Spec.Triggers)
}

// GetVersion returns a string representing the version
func (af *AbstractFunction) GetVersion() string {
	if af.Config.Spec.Version == -1 {
		return "latest"
	}

	return strconv.Itoa(af.Config.Spec.Version)
}
