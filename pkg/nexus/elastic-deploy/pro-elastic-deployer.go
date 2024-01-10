package elastic_deploy

import (
	"fmt"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/nexus/common/env"
	"github.com/nuclio/nuclio/pkg/nexus/elastic-deploy/docker"
)

type ProElasticDeploy struct {
	envRegistry  *env.EnvRegistry
	deployer     ElasticDeployer
	dockerClient dockerclient.Client
}

func NewProElasticDeploy(envRegistry *env.EnvRegistry, client dockerclient.Client) *ProElasticDeploy {
	return &ProElasticDeploy{
		envRegistry:  envRegistry,
		dockerClient: client,
	}
}

func (ped *ProElasticDeploy) getBaseContainerName() string {
	return "nuclio-" + string(ped.envRegistry.NuclioNamespace) + "-"
}

func (ped *ProElasticDeploy) Initialize() {
	fmt.Println(ped.envRegistry.NuclioEnvironment)
	if ped.envRegistry.NuclioEnvironment == "local" {
		ped.deployer = docker.NewDockerDeployer(ped.getBaseContainerName())
	} else {
		//ped.envRegistry = env.NewEnvRegistry()
	}
	ped.deployer.Initialize()
}

func (ped *ProElasticDeploy) Start(functionName string) error {
	fmt.Println("Starting function...")
	return ped.deployer.Start(functionName)
}

func (ped *ProElasticDeploy) Pause(functionName string) error {
	return ped.deployer.Pause(functionName)
}

func (ped *ProElasticDeploy) IsRunning(functionName string) bool {
	return ped.deployer.IsRunning(functionName)
}
