package deployer_models

import (
	"time"
)

type ElasticDeployer interface {
	Unpause(functionName string) error
	Pause(functionName string) error
	IsRunning(functionName string) bool
	Initialize()
	GetNuclioFunctionContainer() (*[]string, error)
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
