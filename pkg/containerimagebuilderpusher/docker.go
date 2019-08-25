package containerimagebuilderpusher

import (
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/logger"
)

type Docker struct {
	dockerClient dockerclient.Client
	logger       logger.Logger
}

func NewDocker(logger logger.Logger) (*Docker, error) {

	dockerClient, err := dockerclient.NewShellClient(logger, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create docker client")
	}

	dockerBuilder := &Docker{
		dockerClient: dockerClient,
		logger:       logger,
	}

	return dockerBuilder, nil
}

func (d *Docker) BuildAndPushContainerImage(buildOptions *BuildOptions, namespace string) error {

	err := d.buildContainerImage(buildOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to build docker image")
	}

	err = d.pushContainerImage(buildOptions.Image, buildOptions.RegistryURL)
	if err != nil {
		return errors.Wrap(err, "Failed to push docker image into registry")
	}

	err = d.saveContainerImage(buildOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to save docker image")
	}

	d.logger.DebugWith("Docker image was successfully built and pushed into docker registry", "image", buildOptions.Image)

	return nil
}

func (d *Docker) buildContainerImage(buildOptions *BuildOptions) error {

	d.logger.DebugWith("Building docker image", "image", buildOptions.Image)

	return d.dockerClient.Build(&dockerclient.BuildOptions{
		ContextDir:     buildOptions.ContextDir,
		Image:          buildOptions.Image,
		DockerfilePath: buildOptions.DockerfilePath,
		NoCache:        buildOptions.NoCache,
		BuildArgs:      buildOptions.BuildArgs,
	})

}

func (d *Docker) pushContainerImage(image string, registryURL string) error {
	d.logger.DebugWith("Pushing docker image into registry",
		"image", image,
		"registry", registryURL)

	if registryURL != "" {
		return d.dockerClient.PushImage(image, registryURL)
	}

	return nil
}

func (d *Docker) saveContainerImage(buildOptions *BuildOptions) error {
	var err error

	if buildOptions.OutputImageFile != "" {
		d.logger.InfoWith("Saving built docker image as archive", "outputFile", buildOptions.OutputImageFile)
		err = d.dockerClient.Save(buildOptions.Image, buildOptions.OutputImageFile)
	}

	return err
}
