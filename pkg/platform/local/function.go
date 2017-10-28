package local

import (
	"strconv"

	"github.com/nuclio/nuclio/pkg/dockerclient"
)

type function struct {
	dockerclient.Container
}

// Initialize does nothing, seeing how no fields require lazy loading
func (f *function) Initialize([]string) error {
	return nil
}

// GetNamespace returns the namespace of the function, if its part of a namespace
func (f *function) GetNamespace() string {
	return "n/a"
}

// GetName returns the name of the function
func (f *function) GetName() string {
	return f.Container.Config.Labels["nuclio-function-name"]
}

// GetVersion returns the version of the function
func (f *function) GetVersion() string {
	return "latest"
}

// GetState returns the state of the function
func (f *function) GetState() string {
	return "RUNNING"
}

// GetHTTPPort returns the port of the HTTP trigger
func (f *function) GetHTTPPort() int {
	port, _ := strconv.Atoi(f.Container.HostConfig.PortBindings["8080/tcp"][0].HostPort)
	return port
}

// GetLabels returns the function labels
func (f *function) GetLabels() map[string]string {
	return f.Container.Config.Labels
}

// GetReplicas returns the current # of replicas and the configured # of replicas
func (f *function) GetReplicas() (int, int) {
	return 1, 1
}

// GetClusterIP gets the IP of the cluster hosting the function
func (f *function) GetClusterIP() string {
	return "localhost"
}
