package config

import "sync/atomic"

type NexusConfig struct {
	MaxParallelRequests      *atomic.Int32
	FunctionExecutionChannel chan string
}

func NewNexusConfig(maxParallelRequests *atomic.Int32, executionChannel chan string) NexusConfig {
	return NexusConfig{
		MaxParallelRequests:      maxParallelRequests,
		FunctionExecutionChannel: executionChannel,
	}
}

func NewDefaultNexusConfig() NexusConfig {
	var maxParallelRequests atomic.Int32
	maxParallelRequests.Store(200)

	channel := make(chan string, maxParallelRequests.Load()*10)
	return NewNexusConfig(&maxParallelRequests, channel)
}
