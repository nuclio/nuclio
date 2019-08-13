package dockerbuilder

import "github.com/nuclio/nuclio/pkg/dockerclient"

// DockerBuilder is a builder of docker images
type DockerBuilder interface {

	// BuildAndPushDockerImage builds docker image and pushes it into docker registry
	BuildAndPushDockerImage(buildOptions *dockerclient.BuildOptions, namespace string) error
}
