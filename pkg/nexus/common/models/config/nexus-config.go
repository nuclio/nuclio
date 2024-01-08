package config

import "sync/atomic"

type NexusConfig struct {
	MaxParallelRequests *atomic.Int32
}

func NewNexusConfig(maxParallelRequests *atomic.Int32) NexusConfig {
	return NexusConfig{
		MaxParallelRequests: maxParallelRequests,
	}
}

func NewDefaultNexusConfig() NexusConfig {
	var maxParallelRequests atomic.Int32
	maxParallelRequests.Store(200)
	return NewNexusConfig(&maxParallelRequests)
}
