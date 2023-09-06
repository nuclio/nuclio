/*
Copyright 2023 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package containerimagebuilderpusher

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/rs/xid"
)

const (
	artifactDirNameInStaging = "artifacts"
)

type Docker struct {
	dockerClient         dockerclient.Client
	logger               logger.Logger
	builderConfiguration *ContainerBuilderConfiguration
}

func NewDocker(logger logger.Logger, builderConfiguration *ContainerBuilderConfiguration) (*Docker, error) {

	dockerClient, err := dockerclient.NewShellClient(logger, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create docker client")
	}

	dockerBuilder := &Docker{
		dockerClient:         dockerClient,
		logger:               logger,
		builderConfiguration: builderConfiguration,
	}

	return dockerBuilder, nil
}

func (d *Docker) GetKind() string {
	return "docker"
}

func (d *Docker) BuildAndPushContainerImage(ctx context.Context, buildOptions *BuildOptions, namespace string) error {

	if err := d.gatherArtifactsForSingleStageDockerfile(ctx, buildOptions); err != nil {
		return errors.Wrap(err, "Failed to build image artifacts")
	}

	if err := d.buildContainerImage(ctx, buildOptions); err != nil {
		return errors.Wrap(err, "Failed to build docker image")
	}

	if err := d.pushContainerImage(ctx, buildOptions.Image, buildOptions.RegistryURL); err != nil {
		return errors.Wrap(err, "Failed to push docker image into registry")
	}

	if err := d.saveContainerImage(ctx, buildOptions); err != nil {
		return errors.Wrap(err, "Failed to save docker image")
	}

	d.logger.InfoWithCtx(ctx,
		"Docker image was successfully built and pushed into docker registry",
		"image", buildOptions.Image)

	return nil
}

func (d *Docker) GetOnbuildStages(onbuildArtifacts []runtime.Artifact) ([]string, error) {

	// Currently docker builder doesn't utilize multistage docker builds
	return []string{}, nil
}

func (d *Docker) GetDefaultRegistryCredentialsSecretName() string {
	return d.builderConfiguration.DefaultRegistryCredentialsSecretName
}

func (d *Docker) GetRegistryKind() string {
	return d.builderConfiguration.RegistryKind
}

func (d *Docker) TransformOnbuildArtifactPaths(onbuildArtifacts []runtime.Artifact) (map[string]string, error) {

	// maps between a _relative_ path in staging to the path in the image
	relativeOnbuildArtifactPaths := map[string]string{}
	for _, onbuildArtifact := range onbuildArtifacts {
		for localArtifactPath, imageArtifactPath := range onbuildArtifact.Paths {
			relativeArtifactPathInStaging := path.Join(artifactDirNameInStaging, path.Base(localArtifactPath))
			relativeOnbuildArtifactPaths[relativeArtifactPathInStaging] = imageArtifactPath
		}
	}

	return relativeOnbuildArtifactPaths, nil
}

func (d *Docker) GetBaseImageRegistry(registry string) string {
	return d.builderConfiguration.DefaultBaseRegistryURL
}

func (d *Docker) GetOnbuildImageRegistry(registry string) string {
	return d.builderConfiguration.DefaultOnbuildRegistryURL
}

func (d *Docker) buildContainerImage(ctx context.Context, buildOptions *BuildOptions) error {

	d.logger.InfoWithCtx(ctx, "Building docker image", "image", buildOptions.Image)

	return d.dockerClient.Build(&dockerclient.BuildOptions{
		ContextDir:     buildOptions.ContextDir,
		Image:          buildOptions.Image,
		DockerfilePath: buildOptions.DockerfileInfo.DockerfilePath,
		NoCache:        buildOptions.NoCache,
		Pull:           buildOptions.Pull,
		BuildArgs:      buildOptions.BuildArgs,
		BuildFlags:     buildOptions.BuildFlags,
	})

}

func (d *Docker) pushContainerImage(ctx context.Context, image string, registryURL string) error {
	d.logger.InfoWithCtx(ctx,
		"Pushing docker image into registry",
		"image", image,
		"registry", registryURL)

	if registryURL != "" {
		return d.dockerClient.PushImage(image, registryURL)
	}

	return nil
}

func (d *Docker) saveContainerImage(ctx context.Context, buildOptions *BuildOptions) error {
	if buildOptions.OutputImageFile != "" {
		d.logger.InfoWithCtx(ctx, "Archiving built docker image", "OutputImageFile", buildOptions.OutputImageFile)
		return d.dockerClient.Save(buildOptions.Image, buildOptions.OutputImageFile)
	}
	return nil
}

func (d *Docker) ensureImagesExist(ctx context.Context,
	buildOptions *BuildOptions, images []string) error {
	if buildOptions.NoBaseImagePull {
		d.logger.DebugWithCtx(ctx,
			"Skipping base images pull", "images", images)
		return nil
	}

	for _, image := range images {
		if err := d.dockerClient.PullImage(image); err != nil {
			return errors.Wrap(err, "Failed to pull image")
		}
	}

	return nil
}

func (d *Docker) gatherArtifactsForSingleStageDockerfile(ctx context.Context,
	buildOptions *BuildOptions) error {
	artifactsDir := path.Join(buildOptions.ContextDir, artifactDirNameInStaging)

	// create an artifacts directory to which we'll copy all of our stuff
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		return errors.Wrap(err, "Failed to create artifacts directory")
	}

	// maps between a path in the onbuild image to a local path in artifacts
	for _, onbuildArtifact := range buildOptions.DockerfileInfo.OnbuildArtifacts {

		// to facilitate good ux, pull images that we're going to need (and log it) before copying
		// objects from them. this also prevents docker spewing out errors about an image not existing
		if err := d.ensureImagesExist(ctx, buildOptions, []string{onbuildArtifact.Image}); err != nil {
			return errors.Wrap(err, "Failed to ensure required images exist")
		}

		onbuildArtifactPaths := map[string]string{}
		for source := range onbuildArtifact.Paths {
			onbuildArtifactPaths[source] = path.Join(artifactsDir, path.Base(source))
		}

		if onbuildArtifact.ExternalImage {

			// For existing images - just copy the artifacts
			if err := d.dockerClient.CopyObjectsFromImage(onbuildArtifact.Image,
				onbuildArtifactPaths,
				false); err != nil {
				return errors.Wrap(err, "Failed to copy artifact from external image")
			}
		}

		// build an image to trigger the onbuild stuff. then extract the artifacts
		if err := d.buildFromAndCopyObjectsFromContainer(ctx,
			onbuildArtifact.Image,
			buildOptions.ContextDir,
			onbuildArtifactPaths,
			buildOptions.BuildArgs); err != nil {
			return errors.Wrap(err, "Failed to copy objects from onbuild")
		}
	}

	return nil
}

func (d *Docker) buildFromAndCopyObjectsFromContainer(ctx context.Context,
	onbuildImage string,
	contextDir string,
	artifactPaths map[string]string,
	buildArgs map[string]string) error {

	dockerfilePath := path.Join(contextDir, "Dockerfile.onbuild")

	onbuildDockerfileContents := fmt.Sprintf(`FROM %s
ARG NUCLIO_LABEL
ARG NUCLIO_ARCH
`, onbuildImage)

	// generate a simple Dockerfile from the onbuild image
	if err := os.WriteFile(dockerfilePath, []byte(onbuildDockerfileContents), 0644); err != nil {
		return errors.Wrapf(err, "Failed to write onbuild Dockerfile to %s", dockerfilePath)
	}

	// log
	d.logger.DebugWithCtx(ctx,
		"Generated onbuild Dockerfile",
		"contents", onbuildDockerfileContents)

	// generate an image name
	onbuildImageName := fmt.Sprintf("nuclio-onbuild-%s", xid.New().String())

	// trigger a build
	if err := d.dockerClient.Build(&dockerclient.BuildOptions{
		Image:          onbuildImageName,
		ContextDir:     contextDir,
		BuildArgs:      buildArgs,
		DockerfilePath: dockerfilePath,
	}); err != nil {
		return errors.Wrap(err, "Failed to build onbuild image")
	}

	defer d.dockerClient.RemoveImage(onbuildImageName) // nolint: errcheck

	// now that we have an image, we can copy the artifacts from it
	return d.dockerClient.CopyObjectsFromImage(onbuildImageName, artifactPaths, false)
}
