package build

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/nuclio-build/util"
	"github.com/nuclio/nuclio/pkg/util/cmdrunner"

	"github.com/pkg/errors"
)

const (
	onBuildImageName       = "nuclio/nuclio:onbuild"
	builderOutputImageName = "nuclio/builder-output"
)

type dockerHelper struct {
	logger    nuclio.Logger
	cmdRunner *cmdrunner.CmdRunner
	env       *env
}

type buildOptions struct {
	Tag        string
	Dockerfile string
}

func newDockerHelper(parentLogger nuclio.Logger, env *env) (*dockerHelper, error) {
	var err error

	b := &dockerHelper{
		logger: parentLogger.GetChild("docker").(nuclio.Logger),
		env:    env,
	}

	// set cmdrunner
	b.cmdRunner, err = cmdrunner.NewCmdRunner(env.logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create command runner")
	}

	_, err = b.cmdRunner.Run(nil, "docker version")
	if err != nil {
		return nil, errors.Wrap(err, "No docker client found")
	}

	return b, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

func (d *dockerHelper) prepareBuildContext(name string, paths []string) (string, error) {
	buildContext := filepath.Join(d.env.getWorkDir(), name)
	d.logger.DebugWith("Preparing build context", "name", name, "path", buildContext)
	if err := os.MkdirAll(buildContext, 0766); err != nil {
		return "", errors.Wrapf(err, "Can't create context dir %s", buildContext)
	}

	for _, path := range paths {
		d.logger.DebugWith("Adding path to build context", "path", path)
		if isDir(path) {
			// Copy all paths under path to root
			// (we can't CopyDir(src, dest) since it'll create one level too much)
			entries, err := ioutil.ReadDir(path)
			if err != nil {
				return "", errors.Wrapf(err, "Can't read directory %q", path)
			}
			for _, info := range entries {
				src := filepath.Join(path, info.Name())
				dest := filepath.Join(buildContext, info.Name())
				if info.IsDir() {
					if err := util.CopyDir(src, dest); err != nil {
						return "", errors.Wrapf(err, "Can't copy %q -> %q", path, dest)
					}
				} else {
					if err := util.CopyFile(src, dest); err != nil {
						return "", errors.Wrapf(err, "Can't copy %q -> %q", path, dest)
					}
				}
			}
		} else { // File at top level
			dest := filepath.Join(buildContext, filepath.Base(path))
			if err := util.CopyFile(path, dest); err != nil {
				return "", errors.Wrapf(err, "Can't copy %q -> %q", path, dest)
			}
		}
	}

	return buildContext, nil
}

func (d *dockerHelper) doBuild(image string, buildContext string, opts *buildOptions) error {
	d.logger.DebugWith("Building image", "image", image)

	var err error
	runOpts := &cmdrunner.RunOptions{WorkingDir: &buildContext}
	if opts == nil {
		_, err = d.cmdRunner.Run(runOpts, "docker build -t %s .", image)
	} else {
		_, err = d.cmdRunner.Run(runOpts, "docker build -t %s -f %s .", opts.Tag, opts.Dockerfile)
	}

	if err != nil {
		return errors.Wrap(err, "Cannot build")
	}

	return nil
}

func (d *dockerHelper) createOnBuildImage() error {
	buildDir := "onbuild"
	buildContextPaths := []string{
		filepath.Join(d.env.getNuclioDir(), "hack", "processor", "build", buildDir),
	}

	buildContext, err := d.prepareBuildContext("nuclio-on-build", buildContextPaths)
	if err != nil {
		return errors.Wrap(err, "Error trying to prepare onbuild context")
	}

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

	options := buildOptions{
		Tag:        builderOutputImageName,
		Dockerfile: filepath.Join("hack", "processor", "build", "builder", "Dockerfile"),
	}
	return d.doBuild(builderOutputImageName, buildContext, &options)
}

func (d *dockerHelper) createBinaryContainer() (string, error) {
	d.logger.DebugWith("Creating container for image", "name", builderOutputImageName)

	out, err := d.cmdRunner.Run(nil, "docker create %s", builderOutputImageName)
	if err != nil {
		return "", errors.Wrap(err, "Unable to create builder container.")
	}

	return strings.TrimSpace(out), nil
}

func (d *dockerHelper) createBuilderImage() error {
	if err := d.buildBuilder(); err != nil {
		return err
	}

	dockerContainerID, err := d.createBinaryContainer()
	if err != nil {
		return err
	}

	binaryPath := "/go/bin/processor"
	destDir := d.env.getWorkDir()
	d.logger.DebugWith("Copying binary from container", "container", dockerContainerID, "path", binaryPath, "target", destDir)
	_, err = d.cmdRunner.Run(nil, "docker cp %s:%s %s", dockerContainerID, binaryPath, destDir)
	if err != nil {
		return errors.Wrap(err, "Can't copy from container")
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
		return errors.Wrap(err, "Error preparing output build context")
	}

	dockerfile := "Dockerfile.alpine"
	if len(d.env.config.Build.Packages) > 0 {
		dockerfile = "Dockerfile.jessie"
	}

	options := buildOptions{
		Tag:        d.env.outputName,
		Dockerfile: filepath.Join("hack", "processor", "build", dockerfile),
	}
	err = d.doBuild(d.env.outputName, buildContext, &options)
	if err != nil {
		return errors.Wrap(err, "Failed to build image")
	}

	if d.env.options.PushRegistry != "" {
		return d.pushImage(d.env.outputName, d.env.options.PushRegistry)
	}

	return nil
}

func (d *dockerHelper) cleanupBuilder() {
	out, err := d.cmdRunner.Run(nil, "docker images --format {{.ID}} %s", builderOutputImageName)
	if err != nil {
		d.logger.WarnWith("Can't list images", "image", builderOutputImageName, "error", err)
		return
	}
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		imageID := strings.TrimSpace(scanner.Text())
		if len(imageID) == 0 {
			continue
		}
		d.logger.InfoWith("Deleting image", "id", imageID)
		if _, err := d.cmdRunner.Run(nil, "docker rmi -f %s", imageID); err != nil {
			d.logger.WarnWith("Can't delete image", "error", err)
		}
	}

	if err = scanner.Err(); err != nil {
		d.logger.WarnWith("Can't scan output", "error", err)
	}
}

func (d *dockerHelper) close() {
	d.cleanupBuilder()
}

func (d *dockerHelper) pushImage(imageName, registryURL string) error {
	taggedImageName := registryURL + "/" + imageName

	d.logger.InfoWith("Pushing image", "from", imageName, "to", taggedImageName)

	_, err := d.cmdRunner.Run(nil, "docker tag %s %s", imageName, taggedImageName)
	if err != nil {
		return errors.Wrap(err, "Failed to tag image")
	}

	_, err = d.cmdRunner.Run(nil, "docker push %s", taggedImageName)
	if err != nil {
		return errors.Wrap(err, "Failed to push image")
	}

	return nil
}
