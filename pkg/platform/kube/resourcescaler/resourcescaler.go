package main

import (
	"github.com/v3io/scaler/pkg"
)

type NuclioResourceScaler struct {

}

func New() scaler.ResourceScaler {
	return &NuclioResourceScaler{}
}

func (n *NuclioResourceScaler) SetScale(namespace string, resource scaler.Resource, scale int) error {
	return nil
}

func (n *NuclioResourceScaler) GetResources() ([]scaler.Resource, error) {
	return []scaler.Resource{}, nil
}

func (n *NuclioResourceScaler) GetConfig() (*scaler.ResourceScalerConfig, error) {
	return &scaler.ResourceScalerConfig{}, nil
}
