package build

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/nuclio/nuclio/pkg/nuclio-build/util"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/jhoonb/archivex"
	"github.com/pkg/errors"
	"github.com/nuclio/nuclio-sdk/logger"
)

const (
	onBuildImageName       = "nuclio/nuclio:onbuild"
	builderOutputImageName = "nuclio/builder-output"
)

type dockerHelper struct {
	logger logger.Logger
	env    *env
	client *client.Client
}

func newDockerHelper(parentLogger logger.Logger, env *env) (*dockerHelper, error) {
	b := &dockerHelper{
		logger: parentLogger.GetChild("docker").(logger.Logger),
		env:    env,
	}

	if err := b.init(); err != nil {
		return nil, err
	}

	return b, nil
}

func (d *dockerHelper) init() error {
	d.logger.Debug("Building docker client from environment")

	docker, err := client.NewEnvClient()
	if err != nil {
		return errors.Wrap(err, "Unable to connect to local dockerHelper.")
	} else {
		d.client = docker
	}

	return nil
}

func (d *dockerHelper) prepareBuildContext(name string, paths []string) (*os.File, error) {
	tar := filepath.Join(d.env.getWorkDir(), name+".tar")
	d.logger.DebugWith("Preparing build context", "name", name, "tar", tar)

	buildContext := &archivex.TarFile{}
	buildContext.Create(tar)

	for _, path := range paths {
		d.logger.DebugWith("Adding path to build context", "path", path)
		buildContext.AddAll(path, false)
	}

	if err := buildContext.Close(); err != nil {
		return nil, errors.Wrapf(err, "Error creating tar %q", tar)
	}

	file, err := os.Open(tar)
	if err != nil {
		return nil, errors.Wrapf(err, "Can't open tar file %q", tar)
	}

	return file, nil
}

func (d *dockerHelper) doBuild(image string, buildContext io.Reader, opts *types.ImageBuildOptions) error {
	d.logger.DebugWith("Building image", "image", image)

	if opts == nil {
		opts = &types.ImageBuildOptions{Tags: []string{image}}
	}

	resp, err := d.client.ImageBuild(context.Background(), buildContext, *opts)
	if err != nil {
		return errors.Wrap(err, "Image building encounter error")
	} else {
		defer resp.Body.Close()

		content, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		progress := string(content)
		d.logger.DebugWith("Got image progress", "image", image, "progress", progress)

		if strings.Contains(progress, `"error"`) && strings.Contains(progress, "\"errorDetail\"") {
			return fmt.Errorf("Encounter build error.\n%s", progress)
		}
	}

	return nil
}

func (d *dockerHelper) createOnBuildImage() error {
	buildContextPaths := []string{
		filepath.Join(d.env.getNuclioDir(), "hack/processor/build/onbuild"),
	}

	buildContext, err := d.prepareBuildContext("nuclio-on-build", buildContextPaths)
	if err != nil {
		return errors.Wrap(err, "Error trying to prepare onbuild context")
	}

	defer buildContext.Close()

	return d.doBuild(onBuildImageName, buildContext, nil)
}

func (d *dockerHelper) buildBuilder() error {
	buildContextPaths := []string{
		d.env.getNuclioDir(),
	}

	buildContext, err := d.prepareBuildContext("nuclio-builder-output", buildContextPaths)
	if err != nil {
		return errors.Wrap(err, "Error trying to prepare build context for builder")
	}

	defer buildContext.Close()

	return d.doBuild(builderOutputImageName, buildContext, &types.ImageBuildOptions{
		Tags:       []string{builderOutputImageName},
		Dockerfile: filepath.Join("hack", "processor", "build", "builder", "Dockerfile"),
	})
}

func (d *dockerHelper) createBinaryContainer() (string, error) {
	d.logger.DebugWith("Creating container for image", "name", builderOutputImageName)

	dockerContainer, err := d.client.ContainerCreate(context.Background(), &container.Config{Image: builderOutputImageName}, nil, nil, "")
	if err != nil {
		return "", errors.Wrap(err, "Unable to create builder container.")
	}

	return dockerContainer.ID, nil
}

func (d *dockerHelper) createBuilderImage() error {
	if err := d.buildBuilder(); err != nil {
		return err
	}

	d.cleanupBuilder()

	dockerContainerID, err := d.createBinaryContainer()
	if err != nil {
		return err
	}

	binaryPath := "/go/bin/processor"
	d.logger.DebugWith("Copying binary from container", "container", dockerContainerID, "path", binaryPath)

	reader, _, err := d.client.CopyFromContainer(context.Background(), dockerContainerID, binaryPath)
	if err != nil {
		return errors.Wrap(err, "Failure to read from container.")
	}

	defer reader.Close()
	d.logger.DebugWith("Untaring binary", "dest", d.env.getBinaryPath())

	err = util.UnTar(reader, d.env.getWorkDir())
	if err != nil {
		return errors.Wrap(err, "Failure to read file from container.")
	}

	return nil
}

func (d *dockerHelper) createProcessorImage() error {
	if err := os.MkdirAll(filepath.Join(d.env.getNuclioDir(), "bin"), 0755); err != nil {
		return errors.Wrapf(err, "Unable to mkdir for bin output")
	}

	processorOutput := filepath.Join(d.env.getNuclioDir(), "bin", "processor")

	if err := util.CopyFile(d.env.getBinaryPath(), processorOutput); err != nil {
		return errors.Wrapf(err, "Unable to copy file %s to %s", d.env.getBinaryPath(), processorOutput)
	}

	buildContextPaths := []string{
		d.env.getNuclioDir(),
		filepath.Join(d.env.userFunctionPath, d.env.config.Name), // function path in temp
	}

	buildContext, err := d.prepareBuildContext("nuclio-output", buildContextPaths)
	if err != nil {
		return errors.Wrapf(err, "Error preparing output build context")
	}

	defer buildContext.Close()

	dockerfile := "Dockerfile.alpine"
	if len(d.env.config.Build.Packages) > 0 {
		dockerfile = "Dockerfile.jessie"
	}

	return d.doBuild(d.env.outputName, buildContext, &types.ImageBuildOptions{
		Tags:       []string{d.env.outputName},
		Dockerfile: filepath.Join("hack", "processor", "build", dockerfile),
	})
}

func (d *dockerHelper) cleanupBuilder() {
	args := filters.NewArgs()
	args.Add("image.name", builderOutputImageName)

	existing, err := d.client.ContainerList(context.Background(), types.ContainerListOptions{
		Filters: args,
	})

	if err == nil && len(existing) > 0 {
		d.logger.DebugWith("Found containers matching name", "num", len(existing), "name", builderOutputImageName)
		for _, exists := range existing {
			d.client.ContainerRemove(context.Background(), exists.ID, types.ContainerRemoveOptions{Force: true})
		}
	}
}

func (d *dockerHelper) close() {
	d.cleanupBuilder()
}
