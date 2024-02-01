package env

import (
	"fmt"
	"os"
)

// EnvRegistry is a registry for environment variables. In this component all environment variables are defined and read.
type EnvRegistry struct {
	NuclioEnvironment EnvVariable // local -- docker, kube
	NuclioNamespace   EnvVariable
}

type EnvVariable string

// The environment variables that are used by the Nexus.
const (
	deployEnvironment EnvVariable = "DEPLOY_ENVIRONMENT"
	nuclioNamespace   EnvVariable = "NUCTL_NAMESPACE"
)

// NewEnvRegistry creates a new EnvRegistry.
func NewEnvRegistry() *EnvRegistry {
	return &EnvRegistry{}
}

// Initialize reads the environment variables and sets the values in the EnvRegistry.
func (er *EnvRegistry) Initialize() {
	deployEnvValue := os.Getenv(string(deployEnvironment))
	if deployEnvValue == "" {
		fmt.Println("DEPLOY_ENVIRONMENT is not set, defaulting to kube")
		deployEnvValue = "kube"
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
