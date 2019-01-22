package main

import (
	"github.com/nuclio/logger"
	"github.com/v3io/scaler/pkg"
)

type NuclioResourceScaler struct {
	
}

func New() interface{} {
	return 90
}

func (n *NuclioResourceScaler) SetScale(logger logger.Logger,
	namespace string,
	resource scaler.Resource,
	scale int) error {
	return nil
}

func (n *NuclioResourceScaler) GetResources(namespace string) ([]scaler.Resource, error) {
	return []scaler.Resource{}, nil
}

func (n *NuclioResourceScaler) GetConfig() (*scaler.ResourceScalerConfig, error) {
	return &scaler.ResourceScalerConfig{}, nil
}
