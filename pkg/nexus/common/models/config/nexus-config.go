package config

import "sync/atomic"

// NexusConfig defines the configuration for the nexus. This allows to fine tune the nexus.
type NexusConfig struct {
	MaxParallelRequests *atomic.Int32
}

// NewNexusConfig allows to create a nexus config.
func NewNexusConfig(maxParallelRequests *atomic.Int32) NexusConfig {
	return NexusConfig{
		MaxParallelRequests: maxParallelRequests,
	}
}

// NewDefaultNexusConfig allows to create a nexus config with default values.
// MaxParallelRequests is set to 200.
func NewDefaultNexusConfig() NexusConfig {
	var maxParallelRequests atomic.Int32
	maxParallelRequests.Store(200)
	return NewNexusConfig(&maxParallelRequests)
}
