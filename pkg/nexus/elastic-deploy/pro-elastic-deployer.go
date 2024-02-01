package elastic_deploy

import (
	"fmt"
	"github.com/nuclio/nuclio/pkg/nexus/common/env"
	"github.com/nuclio/nuclio/pkg/nexus/elastic-deploy/docker"
	"github.com/nuclio/nuclio/pkg/nexus/elastic-deploy/minikube"
	"github.com/nuclio/nuclio/pkg/nexus/elastic-deploy/models"
	"sync"
	"time"
)

// ProElasticDeploy is the general deployer, which allows the scheduler to unpause functions
// It automatically pauses functions after a given duration to save resources
type ProElasticDeploy struct {
	deployer_models.ProElasticDeployerConfig

	envRegistry                *env.EnvRegistry
	deployer                   deployer_models.ElasticDeployer
	durationFunctionsContainer *map[string]time.Time
	lock                       *sync.RWMutex
}

// NewProElasticDeploy creates a new pro elastic deployer
func NewProElasticDeploy(envRegistry *env.EnvRegistry, config deployer_models.ProElasticDeployerConfig) *ProElasticDeploy {
	return &ProElasticDeploy{
		envRegistry:              envRegistry,
		ProElasticDeployerConfig: config,
		lock:                     &sync.RWMutex{},
	}
}

// NewProElasticDeployDefault creates a new pro elastic deployer with default config
func NewProElasticDeployDefault(envRegistry *env.EnvRegistry) *ProElasticDeploy {
	deployConfig := deployer_models.NewProElasticDeployerConfig(5*time.Second, 5*time.Second)
	return NewProElasticDeploy(envRegistry, deployConfig)
}

// getBaseContainerName returns all nuclio function container
func (ped *ProElasticDeploy) getBaseContainerName() string {
	return "nuclio-" + string(ped.envRegistry.NuclioNamespace) + "-"
}

// Initialize initializes the pro elastic deployer for the right environment
func (ped *ProElasticDeploy) Initialize() {
	dfc := make(map[string]time.Time)
	if ped.envRegistry.NuclioEnvironment == "local" {
		ped.deployer = docker.NewDockerDeployer(ped.getBaseContainerName(), &ped.ProElasticDeployerConfig)
	} else if ped.envRegistry.NuclioEnvironment == "kube" {
		fmt.Printf("Kube deployer is not implemented yet")
		ped.deployer = minikube.NewMinikubeDeployer(ped.getBaseContainerName(), &ped.ProElasticDeployerConfig, &dfc)
	}

	ped.durationFunctionsContainer = &dfc
	ped.deployer.Initialize()
}

// Unpause unpauses a function to allow synchronous requests to be sent to the container
func (ped *ProElasticDeploy) Unpause(functionName string) error {
	err := ped.deployer.Unpause(functionName)
	if err != nil {
		return err
	}

	for !ped.deployer.IsRunning(functionName) {
		time.Sleep(100 * time.Millisecond)
		fmt.Println("Waiting for function container to start...")
	}

	pauseTime := time.Now().Add(ped.MaxIdleTime)
	ped.lock.Lock()
	(*ped.durationFunctionsContainer)[functionName] = pauseTime
	ped.lock.Unlock()
	return nil
}

// PauseUnusedFunctionContainers pauses unused function containers to save resources
// It is called in a cron manner in the background every CheckRemainingTime
func (ped *ProElasticDeploy) PauseUnusedFunctionContainers() {
	for {
		for functionName, remainingDuration := range *ped.durationFunctionsContainer {

			if remainingDuration.Before(time.Now()) {
				err := ped.deployer.Pause(functionName)
				if err != nil {
					fmt.Printf("Error unpausing function: %s", err)
				}
				ped.lock.Lock()
				delete(*ped.durationFunctionsContainer, functionName)
				ped.lock.Unlock()
			}
		}

		time.Sleep(ped.CheckRemainingTime)
	}
}

func (ped *ProElasticDeploy) IsRunning(functionName string) bool {
	return ped.deployer.IsRunning(functionName)
}
