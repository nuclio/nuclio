package minikube

import (
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	deployer_models "github.com/nuclio/nuclio/pkg/nexus/elastic-deploy/models"
	"time"
)

type MinkubeDeployer struct {
	*deployer_models.ProElasticDeployerConfig
	// k8s.Client

	baseContainerName          string
	durationFunctionsContainer *map[string]time.Time
}

func NewMinikubeDeployer(baseContainerName string, config *deployer_models.ProElasticDeployerConfig, durationFunctionsContainer *map[string]time.Time) *MinkubeDeployer {
	return &MinkubeDeployer{
		baseContainerName:          baseContainerName,
		ProElasticDeployerConfig:   config,
		durationFunctionsContainer: durationFunctionsContainer,
	}
}

func (ds *MinkubeDeployer) Initialize() {
	fmt.Printf("Initializing MinkubeDeployer\n")
}

func (ds *MinkubeDeployer) Unpause(functionName string) error {
	fmt.Printf("Unpausing functions has not been implemented yet\n")
	return nil
}

func (ds *MinkubeDeployer) Pause(functionName string) error {
	fmt.Printf("Pausing functions has not been implemented yet\n")
	return nil
}

func (ds *MinkubeDeployer) IsRunning(functionName string) bool {
	fmt.Printf("IsRunning has not been implemented yet\n")
	return true
}

func (ds *MinkubeDeployer) getFunctionContainer(functionName string) *docker.APIContainers {
	fmt.Printf("getFunctionContainer has not been implemented yet\n")
	return nil
}

func (ds *MinkubeDeployer) getContainerName(functionName string) string {
	return ds.baseContainerName + functionName
}
