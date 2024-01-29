package elastic_deploy

import (
	"fmt"
	"github.com/nuclio/nuclio/pkg/nexus/common/env"
	"github.com/nuclio/nuclio/pkg/nexus/elastic-deploy/docker"
	deployer_models "github.com/nuclio/nuclio/pkg/nexus/elastic-deploy/models"
	"time"
)

// ProElasticDeploy is the general deployer, which allows the scheduler to unpause functions
// It automatically pauses functions after a given duration to save resources
type ProElasticDeploy struct {
	deployer_models.ProElasticDeployerConfig

	envRegistry                *env.EnvRegistry
	deployer                   deployer_models.ElasticDeployer
	durationFunctionsContainer *map[string]time.Time
}

// NewProElasticDeploy creates a new pro elastic deployer
func NewProElasticDeploy(envRegistry *env.EnvRegistry, config deployer_models.ProElasticDeployerConfig) *ProElasticDeploy {
	return &ProElasticDeploy{
		envRegistry:              envRegistry,
		ProElasticDeployerConfig: config,
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
	if ped.envRegistry.NuclioEnvironment == "local" {
		dfc := make(map[string]time.Time)

		ped.deployer = docker.NewDockerDeployer(ped.getBaseContainerName(), &ped.ProElasticDeployerConfig, &dfc)
		ped.durationFunctionsContainer = &dfc
	}

	ped.deployer.Initialize()
	fmt.Printf("The durationFunctionContainer is: %s\n", ped.durationFunctionsContainer)
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
	(*ped.durationFunctionsContainer)[functionName] = pauseTime
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
				delete(*ped.durationFunctionsContainer, functionName)
			}
		}

		time.Sleep(ped.CheckRemainingTime)
	}
}
