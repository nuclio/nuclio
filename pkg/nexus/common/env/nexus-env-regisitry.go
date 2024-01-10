package env

import (
	"fmt"
	"os"
)

type EnvRegistry struct {
	NuclioEnvironment EnvVariable // local -- docker, kube
	NuclioNamespace   EnvVariable
}

type EnvVariable string

const (
	deployEnvironment EnvVariable = "DEPLOY_ENVIRONMENT"
	nuclioNamespace   EnvVariable = "NUCTL_NAMESPACE"
)

func NewEnvRegistry() *EnvRegistry {
	return &EnvRegistry{}
}

func (er *EnvRegistry) Initialize() {
	deployEnvValue := os.Getenv(string(deployEnvironment))
	if deployEnvValue == "" {
		fmt.Println("DEPLOY_ENVIRONMENT is not set, defaulting to docker")
		deployEnvValue = "local"
	}
	er.NuclioEnvironment = EnvVariable(deployEnvValue)

	namespaceEnvValue := os.Getenv(string(nuclioNamespace))
	if namespaceEnvValue == "" {
		fmt.Println("NUCTL_NAMESPACE is not set, defaulting to default")
		namespaceEnvValue = "default"
	}
	er.NuclioNamespace = EnvVariable(namespaceEnvValue)

	fmt.Println("The EnvRegistry has been initialized.")
}
