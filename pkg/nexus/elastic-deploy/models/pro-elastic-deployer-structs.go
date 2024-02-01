package deployer_models

import (
	"time"
)

// The following constants are used in the deployer domain throughout utils, helper function, tests and more.
const (
	Running = "running"
	Paused  = "paused"
)

// ElasticDeployer is an interface for a deployer.
type ElasticDeployer interface {
	// Initialize initializes the deployer.
	Initialize()
	// Unpause Resumes a function.
	Unpause(functionName string) error
	// Pause pauses a function to save resources.
	Pause(functionName string) error
	// IsRunning checks if a function is running.
	IsRunning(functionName string) bool
}

type ProElasticDeployerConfig struct {
	MaxIdleTime        time.Duration
	CheckRemainingTime time.Duration
}

func NewProElasticDeployerConfig(maxIdleTime time.Duration, checkRemainingTime time.Duration) ProElasticDeployerConfig {
	return ProElasticDeployerConfig{
		MaxIdleTime:        maxIdleTime,
		CheckRemainingTime: checkRemainingTime,
	}
}
