package build

import (
	"io/ioutil"
	"path/filepath"
	"os"
	"fmt"
	"net/url"
	"path"

	"github.com/nuclio/nuclio/pkg/util/common"
	"github.com/nuclio/nuclio/pkg/processor/config"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/processor/build/util"
	_ "github.com/nuclio/nuclio/pkg/processor/build/runtime/python"

	"github.com/nuclio/nuclio-sdk"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	processorConfigFileName = "processor.yaml"
	buildConfigFileName = "build.yaml"
)

type Builder struct {
	Options

	logger  				nuclio.Logger

	// the handler is a description of the actual entry point into the sources held by the function path.
	functionHandler         string

	// the selected runtimg
	runtime                 runtime.Runtime

	// a temporary directory which contains all the stuff needed to build
	stagingDir              string

	// information about the processor image - the one that actually holds the processor binary and is pushed
	// to the cluster
	processorImage struct {

		// a list of commands that execute when the processor is built
		scriptPathToRunDuringBuild  string

		// a list of commands that execute when the processor is built
		commandsToRunDuringBuild    []string

		// a map of local_path:dest_path. each file / dir from local_path will be copied into
		// the docker image at dest_path
		objectsToCopyDuringBuild    map[string]string

		// the image name we'll base from when we generate the processor image
		baseImageName               string

		// name of the image that will be created
		imageName                   string
	}
}

func NewBuilder(parentLogger nuclio.Logger, options *Options) *Builder {
	return &Builder{
		Options: *options,
		logger:  parentLogger,
	}
}

func (b *Builder) Build() error {
	var err error

	// prepare configuration from both configuration files and things builder infers
	if err := b.readConfiguration(); err != nil {
		return errors.Wrap(err, "Failed to read configuration")
	}

	// create a runtime based on the configuration
	b.runtime, err = b.createRuntime()
	if err != nil {
		return errors.Wrap(err, "Failed create runtime")
	}

	// once we're done reading our configuration, we may still have to fill in the blanks because
	// since the user isn't obligated to always pass all the configuration
	if err := b.enrichConfiguration(); err != nil {
		return errors.Wrap(err, "Failed to enrich configuration")
	}

	// prepare a staging directory
	if err := b.prepareStagingDir(); err != nil {
		return errors.Wrap(err, "Failed to prepare staging dir")
	}

	return nil
}

func (b *Builder) GetFunctionPath() string {
	return b.FunctionPath
}

func (b *Builder) readConfiguration() error {
	var err error

	// resolve the function path - download in case its a URL
	b.FunctionPath, err = b.resolveFunctionPath(b.FunctionPath)
	if err != nil {
		return errors.Wrap(err, "Failed to resolve funciton path")
	}

	if processorConfigPath := b.providedProcessorConfigFilePath(); processorConfigPath != nil {
		if err := b.readProcessorConfigFile(*processorConfigPath); err != nil {
			return errors.Wrap(err, "Failed to read processor configuration")
		}
	}

	if buildConfigPath := b.providedBuildConfigFilePath(); buildConfigPath != nil {
		if err := b.readBuildConfigFile(*buildConfigPath); err != nil {
			return errors.Wrap(err, "Failed to read build configuration")
		}
	}

	return nil
}

func (b *Builder) providedProcessorConfigFilePath() *string {

	// if the user only provided a function file, there's no way he couldn've provided config
	if !common.IsDir(b.FunctionPath) {
		return nil
	}

	processorConfigPath := filepath.Join(b.FunctionPath, processorConfigFileName)

	if !common.FileExists(processorConfigPath) {
		return nil
	}

	return &processorConfigPath
}

func (b *Builder) providedBuildConfigFilePath() *string {

	// if the user only provided a function file, there's no way he couldn've provided config
	if !common.IsDir(b.FunctionPath) {
		return nil
	}

	buildConfigPath := filepath.Join(b.FunctionPath, buildConfigFileName)

	if !common.FileExists(buildConfigPath) {
		return nil
	}

	return &buildConfigPath
}

func (b *Builder) enrichConfiguration() error {

	// if image isn't set, ask runtime
	if b.processorImage.baseImageName == "" {
		b.processorImage.baseImageName = b.runtime.GetDefaultProcessorBaseImage()
	}

	// if the function handler isn't set, ask runtime
	if b.functionHandler == "" {
		functionHandlers, err := b.runtime.DetectFunctionHandlers(b.Options.FunctionPath)
		if err != nil {
			return errors.Wrap(err, "Failed to detect ")
		}

		// use first for now
		b.functionHandler = functionHandlers[0]
	}

	// if output image name isn't set, set it to a derivative of the name
	if b.processorImage.imageName == "" {
		b.processorImage.imageName = fmt.Sprintf("nuclio/processor-%s", b.Options.FunctionName)
	}

	return nil
}

func (b *Builder) resolveFunctionPath(functionPath string) (string, error) {

	// if the function path is a URL - first download the file
	if common.IsURL(functionPath) {
		tempDir, err := ioutil.TempDir("", "")
		if err != nil {
			return "", err
		}

		tempFile, err := common.TempFileSuffix(tempDir, "-handler.go")
		if err != nil {
			return "", err
		}

		tempFileName := tempFile.Name()
		if err := tempFile.Close(); err != nil {
			return "", err
		}

		b.logger.DebugWith("Downloading function",
			"url", functionPath,
			"target", tempFileName)

		if err := common.DownloadFile(functionPath, tempFileName); err != nil {
			return "", err
		}

		return tempFileName, nil
	} else {
		// Assume it's a local path
		return filepath.Abs(filepath.Clean(functionPath))
	}
}

func (b *Builder) readProcessorConfigFile(processorConfigPath string) error {
	processorConfig, err := config.ReadProcessorConfiguration(processorConfigPath)
	if err != nil {
		return err
	}

	functionConfig := processorConfig["function"]
	mapping := map[string]*string{
		"handler": &b.functionHandler,
		"runtime": &b.Runtime,
	}

	for key, builderValue := range mapping {
		valueFromConfig := functionConfig.GetString(key)
		if len(valueFromConfig) == 0 {
			continue
		}

		*builderValue = valueFromConfig
	}

	return nil
}

func (b *Builder) readBuildConfigFile(buildConfigPath string) error {
	v := viper.New()
	v.SetConfigFile(buildConfigPath)

	if err := v.ReadInConfig(); err != nil {
		return errors.Wrapf(err, "Unable to read %q configuration", buildConfigPath)
	}

	// read keys
	b.processorImage.baseImageName = viper.GetString("image")
	b.processorImage.commandsToRunDuringBuild = viper.GetStringSlice("commands")
	b.processorImage.scriptPathToRunDuringBuild = viper.GetString("script")

	return nil
}

func (b *Builder) createRuntime() (runtime.Runtime, error) {
	runtimeName := b.Runtime

	// if runtime isn't set, try to look at extension
	if runtimeName == "" {

		// if the function path is a directory, assume Go for now
		if common.IsDir(b.FunctionPath) {
			runtimeName = "go"
		} else {

			// try to read the file extension (skip dot in extension)
			functionFileExtension := filepath.Ext(b.FunctionPath)[1:]

			// if the file extension is of a known runtime, use that (skip dot in extension)
			switch functionFileExtension {
			case "go":
				runtimeName = "go"
			case "py":
				runtimeName = "python"
			default:
				return nil, fmt.Errorf("No supported runtime for file extension %s", functionFileExtension)
			}
		}
	}

	// if the file extension is of a known runtime, use that
	runtimeFactory, err := runtime.RuntimeRegistrySingleton.Get(runtimeName)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get runtime factory")
	}

	// create a runtime instance
	runtimeInstance, err := runtimeFactory.(runtime.Factory).Create(b.logger, b)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create runtime")
	}

	return runtimeInstance, nil
}

func (b *Builder) prepareStagingDir() error {
	var err error

	// create a staging directory
	b.stagingDir, err = ioutil.TempDir("", "nuclio-build-")
	if err != nil {
		return errors.Wrap(err, "Failed to create staging dir")
	}

	b.logger.DebugWith("Created staging directory", "dir", b.stagingDir)

	// first, tell the specific runtime to do its thing
	if err := b.runtime.OnAfterStagingDirCreated(b.stagingDir); err != nil {
		return errors.Wrap(err, "Failed to prepare staging dir")
	}

	// copy any objects the runtime needs into staging
	if err := b.copyObjectsToStagingDir(); err != nil {
		return errors.Wrap(err, "Failed to prepare staging dir")
	}

	return nil
}

func (b *Builder) copyObjectsToStagingDir() error {
	objectPathsToStagingDir := b.runtime.GetStagingDirObjectPaths()

	//
	if processorConfigPath := b.providedProcessorConfigFilePath(); processorConfigPath != nil {
		objectPathsToStagingDir = append(objectPathsToStagingDir, *processorConfigPath)
	} else {

		// processor config @ the staging dir
		processorConfigStagingPath := path.Join(b.stagingDir, processorConfigFileName)

		// write the contents there
		ioutil.WriteFile(processorConfigStagingPath,
			[]byte(b.runtime.GetProcessorConfigFileContents()),
			os.FileMode(0600))

		b.logger.DebugWith("Generated processor configuration file",
			"path", processorConfigStagingPath,
			"contents", b.runtime.GetProcessorConfigFileContents())
	}

	b.logger.DebugWith("Runtime provided objects to staging dir", "objects", objectPathsToStagingDir)

	// copy the files
	for _, objectPath := range objectPathsToStagingDir {

		// if the object path is a URL, download it
		if common.IsURL(objectPath) {

			// convert to URL
			objectURL, err := url.Parse(objectPath)
			if err != nil {
				return errors.Wrapf(err, "Failed to convert % to URL", objectPath)
			}

			// get the file name
			fileName := path.Base(objectURL.Path)

			// download the file
			if err := common.DownloadFile(objectPath, path.Join(b.stagingDir, fileName)); err != nil {
				return errors.Wrapf(err, "Failed to download %s", objectPath)
			}
		} else {

			objectFileName := path.Base(objectPath)
			destObjectPath := path.Join(b.stagingDir, objectFileName)

			// just copy the file
			if err := util.CopyFile(objectPath, destObjectPath); err != nil {
				return errors.Wrapf(err, "Failed to copy %s to %s", objectPath, b.stagingDir)
			}
		}
	}

	return nil
}
