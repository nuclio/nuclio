package config

import "sync/atomic"

// NexusConfig defines the configuration for the nexus. This allows to fine tune the nexus.
type NexusConfig struct {
	MaxParallelRequests      *atomic.Int32
	FunctionExecutionChannel chan string
}

// NewNexusConfig allows to create a nexus config.
func NewNexusConfig(maxParallelRequests *atomic.Int32, executionChannel chan string) NexusConfig {
	return NexusConfig{
		MaxParallelRequests:      maxParallelRequests,
		FunctionExecutionChannel: executionChannel,
	}
}

// NewDefaultNexusConfig allows to create a nexus config with default values.
// MaxParallelRequests is set to 200.
func NewDefaultNexusConfig() NexusConfig {
	var maxParallelRequests atomic.Int32
	maxParallelRequests.Store(200)

	channel := make(chan string, maxParallelRequests.Load()*10)
	return NewNexusConfig(&maxParallelRequests, channel)
}
