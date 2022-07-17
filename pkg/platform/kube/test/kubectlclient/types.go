package kubectlclient

import "fmt"

type RunKubectlCommandMode string

const (
	RunKubectlCommandDirect   RunKubectlCommandMode = "direct"
	RunKubectlCommandMinikube RunKubectlCommandMode = "minikube"
)

type CommandExecutor interface {
	GetRunCommand() string
	GetKind() RunKubectlCommandMode
}

type minikubeRunOptions struct {
	Profile string
}

func (m *minikubeRunOptions) GetRunCommand() string {
	return fmt.Sprintf("minikube --profile %s kubectl --", m.Profile)
}

func (m *minikubeRunOptions) GetKind() RunKubectlCommandMode {
	return RunKubectlCommandMinikube
}

type directRunOptions struct {
}

func (d *directRunOptions) GetRunCommand() string {
	return "kubectl"
}

func (d *directRunOptions) GetKind() RunKubectlCommandMode {
	return RunKubectlCommandDirect
}
