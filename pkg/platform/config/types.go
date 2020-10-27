package config

import "github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"

type Configuration struct {
	KubeconfigPath                string
	ContainerBuilderConfiguration containerimagebuilderpusher.ContainerBuilderConfiguration
}
