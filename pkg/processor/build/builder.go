/*
Copyright 2017 The Nuclio Authors.

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

package build

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"text/template"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build/inlineparser"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	_ "github.com/nuclio/nuclio/pkg/processor/build/runtime/golang"
	_ "github.com/nuclio/nuclio/pkg/processor/build/runtime/python"
	"github.com/nuclio/nuclio/pkg/processor/build/util"
	"github.com/nuclio/nuclio/pkg/processor/config"
	"github.com/nuclio/nuclio/pkg/util/common"

	"github.com/nuclio/nuclio-sdk"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

const (
	processorConfigFileName             = "processor.yaml"
	buildConfigFileName                 = "build.yaml"
	processorConfigPathInProcessorImage = "/etc/nuclio/processor.yaml"
)

type Builder struct {
	Options

	logger nuclio.Logger

	// the handler is a description of the actual entry point into the sources held by the function path.
	functionHandler string

	// the selected runtimg
	runtime runtime.Runtime

	// a temporary directory which contains all the stuff needed to build
	stagingDir string

	// a docker client with which to build stuff
	dockerClient *dockerclient.Client

	// inline blocks of configuration, having appeared in the source prefixed with @nuclio.<something>
	inlineConfigurationBlock map[string]interface{}

	// information about the processor image - the one that actually holds the processor binary and is pushed
	// to the cluster
	processorImage struct {

		// a list of commands that execute when the processor is built
		scriptPathToRunDuringBuild string

		// a list of commands that execute when the processor is built
		commandsToRunDuringBuild []string

		// a map of local_path:dest_path. each file / dir from local_path will be copied into
		// the docker image at dest_path
		objectsToCopyDuringBuild map[string]string

		// the image name we'll base from when we generate the processor image
		baseImageName string

		// name of the image that will be created
		imageName string

		// the tag of the image that will be created
		imageTag string
	}
}

func NewBuilder(parentLogger nuclio.Logger, options *Options) (*Builder, error) {
	var err error

	newBuilder := &Builder{
		Options: *options,
		logger:  parentLogger,
	}

	newBuilder.dockerClient, err = dockerclient.NewClient(newBuilder.logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create docker client")
	}

	return newBuilder, nil
}

func (b *Builder) Build() (string, error) {
	var err error

	b.logger.InfoWith("Building", "name", b.FunctionName)

	// resolve the function path - download in case its a URL
	b.FunctionPath, err = b.resolveFunctionPath(b.FunctionPath)
	if err != nil {
		return "", errors.Wrap(err, "Failed to resolve funciton path")
	}

	// create a runtime based on the configuration
	b.runtime, err = b.createRuntime()
	if err != nil {
		return "", errors.Wrap(err, "Failed create runtime")
	}

	// parse the inline blocks in the file - blocks of comments starting with @nuclio.<something>. this may be used
	// later on (e.g. for creating files)
	if common.IsFile(b.FunctionPath) {
		b.parseInlineBlocks()
	}

	// prepare configuration from both configuration files and things builder infers
	if err = b.readConfiguration(); err != nil {
		return "", errors.Wrap(err, "Failed to read configuration")
	}

	// once we're done reading our configuration, we may still have to fill in the blanks because
	// since the user isn't obligated to always pass all the configuration
	if err = b.enrichConfiguration(); err != nil {
		return "", errors.Wrap(err, "Failed to enrich configuration")
	}

	// prepare a staging directory
	if err = b.prepareStagingDir(); err != nil {
		return "", errors.Wrap(err, "Failed to prepare staging dir")
	}

	// build the processor image
	processorImageName, err := b.buildProcessorImage()
	if err != nil {
		return "", errors.Wrap(err, "Failed to build processor image")
	}

	// push the processor image
	if err := b.pushProcessorImage(processorImageName); err != nil {
		return "", errors.Wrap(err, "Failed to push processor image")
	}

	b.logger.InfoWith("Build complete")

	return processorImageName, nil
}

func (b *Builder) GetFunctionPath() string {
	return b.FunctionPath
}

func (b *Builder) GetFunctionName() string {
	return b.FunctionName
}

func (b *Builder) GetFunctionHandler() string {
	return b.functionHandler
}

func (b *Builder) GetNuclioSourceDir() string {
	return b.NuclioSourceDir
}

func (b *Builder) GetNuclioSourceURL() string {
	return b.NuclioSourceURL
}

func (b *Builder) GetStagingDir() string {
	return b.stagingDir
}

func (b *Builder) GetFunctionDir() string {

	// if the function directory was passed, just return that. if the function path was passed, return the directory
	// the function is in
	if common.IsDir(b.FunctionPath) {
		return b.FunctionPath
	}

	return path.Dir(b.FunctionPath)
}

func (b *Builder) GetNoBaseImagePull() bool {
	return b.NoBaseImagePull
}

func (b *Builder) readConfiguration() error {

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

	// if the user only provided a function file, check if it had a processor configuration file
	// in an inline configuration block (@nuclio.configure)
	if common.IsFile(b.FunctionPath) {
		inlineProcessorConfig, found := b.inlineConfigurationBlock[processorConfigFileName]
		if !found {
			return nil
		}

		// create a temporary file containing the contents and return that
		processorConfigPath, err := b.createTempFileFromYAML(processorConfigFileName, inlineProcessorConfig)
		if err == nil {
			return &processorConfigPath
		}

		return nil
	}

	processorConfigPath := filepath.Join(b.FunctionPath, processorConfigFileName)

	if !common.FileExists(processorConfigPath) {
		return nil
	}

	return &processorConfigPath
}

func (b *Builder) providedBuildConfigFilePath() *string {

	// if the user only provided a function file, check if it had a processor configuration file
	// in an inline configuration block (@nuclio.configure)
	if common.IsFile(b.FunctionPath) {
		inlineBuildConfig, found := b.inlineConfigurationBlock[buildConfigFileName]
		if !found {
			return nil
		}

		// create a temporary file containing the contents and return that
		buildConfigPath, err := b.createTempFileFromYAML(buildConfigFileName, inlineBuildConfig)
		if err == nil {
			return &buildConfigPath
		}

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
		b.processorImage.baseImageName = b.runtime.GetDefaultProcessorBaseImageName()
	}

	// if the function handler isn't set, ask runtime
	if b.functionHandler == "" {
		functionHandlers, err := b.runtime.DetectFunctionHandlers(b.Options.FunctionPath)
		if err != nil {
			return errors.Wrap(err, "Failed to detect ")
		}

		if len(functionHandlers) == 0 {
			return errors.New("Could not find any handlers")
		}

		// use first for now
		b.functionHandler = functionHandlers[0]
	}

	// if output image name isn't set, set it to a derivative of the name
	if b.processorImage.imageName == "" {
		if b.Options.OutputName == "" {
			b.processorImage.imageName = fmt.Sprintf("nuclio/processor-%s", b.Options.FunctionName)
		} else {
			b.processorImage.imageName = b.Options.OutputName
		}
	}

	// if tag isn't set - use "latest"
	if b.processorImage.imageTag == "" {
		b.processorImage.imageTag = "latest"
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

		tempFileName := path.Join(tempDir, path.Base(functionPath))

		b.logger.DebugWith("Downloading function",
			"url", functionPath,
			"target", tempFileName)

		if err := common.DownloadFile(functionPath, tempFileName); err != nil {
			return "", err
		}

		return tempFileName, nil
	} else {

		// Assume it's a local path
		resolvedPath, err := filepath.Abs(filepath.Clean(functionPath))
		if err != nil {
			return "", errors.Wrap(err, "Failed to get resolve non-url path")
		}

		if !common.FileExists(resolvedPath) {
			return "", fmt.Errorf("Function path doesn't exist: %s", resolvedPath)
		}

		return resolvedPath, nil
	}
}

func (b *Builder) readProcessorConfigFile(processorConfigPath string) error {
	processorConfig, err := config.ReadProcessorConfiguration(processorConfigPath)
	if err != nil {
		return err
	}

	functionConfig := processorConfig["function"]
	if functionConfig == nil {
		return nil
	}

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
	buildConfig := viper.New()
	buildConfig.SetConfigFile(buildConfigPath)

	if err := buildConfig.ReadInConfig(); err != nil {
		return errors.Wrapf(err, "Unable to read %q configuration", buildConfigPath)
	}

	// read keys
	b.processorImage.baseImageName = buildConfig.GetString("image")
	b.processorImage.commandsToRunDuringBuild = buildConfig.GetStringSlice("commands")
	b.processorImage.scriptPathToRunDuringBuild = buildConfig.GetString("script")

	return nil
}

func (b *Builder) createRuntime() (runtime.Runtime, error) {
	runtimeName := b.Runtime

	// if runtime isn't set, try to look at extension
	if runtimeName == "" {

		// if the function path is a directory, assume Go for now
		if common.IsDir(b.FunctionPath) {
			runtimeName = "golang"
		} else {

			// try to read the file extension (skip dot in extension)
			functionFileExtension := filepath.Ext(b.FunctionPath)[1:]

			// if the file extension is of a known runtime, use that (skip dot in extension)
			switch functionFileExtension {
			case "go":
				runtimeName = "golang"
			case "py":
				runtimeName = "python"
			default:
				return nil, fmt.Errorf("No supported runtime for file extension %s", functionFileExtension)
			}
		}

		b.logger.DebugWith("Runtime auto-detected", "runtime", runtimeName)
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

	b.logger.InfoWith("Staging files and preparing base images")

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
	objectPathsToStagingDir := b.runtime.GetProcessorImageObjectPaths()

	// if the function directory holds a processor config, it's common behavior among all
	// the runtimes to copy it to staging so that it can be copied over
	if processorConfigPath := b.providedProcessorConfigFilePath(); processorConfigPath != nil {
		objectPathsToStagingDir[*processorConfigPath] = processorConfigFileName
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

	// copy the files - ignore where we need to copy this in the image, this'll be done later. right now
	// we just want to copy the file from wherever it is to the staging dir root
	for localObjectPath, _ := range objectPathsToStagingDir {

		// if the object path is a URL, download it
		if common.IsURL(localObjectPath) {

			// convert to URL
			objectURL, err := url.Parse(localObjectPath)
			if err != nil {
				return errors.Wrapf(err, "Failed to convert % to URL", localObjectPath)
			}

			// get the file name
			fileName := path.Base(objectURL.Path)

			// download the file
			if err := common.DownloadFile(localObjectPath, path.Join(b.stagingDir, fileName)); err != nil {
				return errors.Wrapf(err, "Failed to download %s", localObjectPath)
			}
		} else if common.IsDir(localObjectPath) {

			if _, err := util.CopyDir(localObjectPath, path.Join(b.stagingDir, path.Base(localObjectPath))); err != nil {
				return err
			}

		} else {
			objectFileName := path.Base(localObjectPath)
			destObjectPath := path.Join(b.stagingDir, objectFileName)

			// if the file is already there, ignore it. this is to allow cases where the user
			// already but the file in staging himself
			if localObjectPath == destObjectPath {
				continue
			}

			// just copy the file
			if err := util.CopyFile(localObjectPath, destObjectPath); err != nil {
				return errors.Wrapf(err, "Failed to copy %s to %s", localObjectPath, destObjectPath)
			}
		}
	}

	return nil
}

func (b *Builder) buildProcessorImage() (string, error) {
	b.logger.InfoWith("Building processor image")

	processorDockerfilePathInStaging, err := b.createProcessorDockerfile()
	if err != nil {
		return "", errors.Wrap(err, "Failed to create processor dockerfile")
	}

	imageName := fmt.Sprintf("%s:%s", b.processorImage.imageName, b.processorImage.imageTag)

	err = b.dockerClient.Build(&dockerclient.BuildOptions{
		ImageName:      imageName,
		DockerfilePath: processorDockerfilePathInStaging,
		NoCache:        true,
	})

	return imageName, err
}

func (b *Builder) createProcessorDockerfile() (string, error) {
	processorDockerfileTemplateFuncs := template.FuncMap{
		"pathBase":      path.Base,
		"isDir":         common.IsDir,
		"objectsToCopy": b.getObjectsToCopyToProcessorImage,
		"baseImageName": func() string { return b.processorImage.baseImageName },
		"commandsToRun": func() []string { return b.processorImage.commandsToRunDuringBuild },
		"configPath":    func() string { return processorConfigPathInProcessorImage },
	}

	processorDockerfileTemplate, err := template.New("").
		Funcs(processorDockerfileTemplateFuncs).
		Parse(processorImageDockerfileTemplate)

	if err != nil {
		return "", errors.Wrap(err, "Failed ot parse processor image Fockerfile template")
	}

	processorDockerfilePathInStaging := filepath.Join(b.stagingDir, "Dockerfile.processor")
	processorDockerfileInStaging, err := os.Create(processorDockerfilePathInStaging)
	if err != nil {
		return "", errors.Wrapf(err, "Can't create processor docker file at %s", processorDockerfilePathInStaging)
	}

	b.logger.DebugWith("Creating Dockerfile from template", "dest", processorDockerfilePathInStaging)

	if err = processorDockerfileTemplate.Execute(processorDockerfileInStaging, nil); err != nil {
		return "", errors.Wrapf(err, "Can't execute template")
	}

	return processorDockerfilePathInStaging, nil
}

// returns a map where key is the relative path into staging of a file that needs
// to be copied into the absolute directory in the processor image (the value of that key).
// processor.yaml is the only file that is expected to be in staging root - all the rest are
// provided by the runtime
func (b *Builder) getObjectsToCopyToProcessorImage() map[string]string {
	objectsToCopyToProcessorImage := map[string]string{
		processorConfigFileName: processorConfigPathInProcessorImage,
	}

	// the runtime specifies key/value where key = absolule local path and
	// value = absolute path into docker. since we already copied these files
	// to the root of staging, we can just take their file name and get relative the
	// path into staging
	for localObjectPath, dockerObjectPath := range b.runtime.GetProcessorImageObjectPaths() {
		objectsToCopyToProcessorImage[path.Base(localObjectPath)] = dockerObjectPath
	}

	return objectsToCopyToProcessorImage
}

// this will parse the source file looking for @nuclio.createFiles blocks. It will then generate these files
// in the staging area
func (b *Builder) parseInlineBlocks() error {

	// create an inline block parser
	parser, err := inlineparser.NewParser(b.logger)
	if err != nil {
		return errors.Wrap(err, "Failed to create parser")
	}

	// create a file reader
	functionFile, err := os.OpenFile(b.FunctionPath, os.O_RDONLY, os.FileMode(0644))
	if err != nil {
		return errors.Wrap(err, "Failed to open function file")
	}

	blocks, err := parser.Parse(functionFile, b.runtime.GetCommentPattern())
	if err != nil {
		return errors.Wrap(err, "Failed to parse inline blocks")
	}

	b.inlineConfigurationBlock, _ = blocks["configure"]

	b.logger.DebugWith("Parsed inline blocks", "configBlock", b.inlineConfigurationBlock)

	return nil
}

// create a temporary file from an unmarshalled YAML
func (b *Builder) createTempFileFromYAML(fileName string, unmarshalledYAMLContents interface{}) (string, error) {
	marshalledFileContents, err := yaml.Marshal(unmarshalledYAMLContents)
	if err != nil {
		return "", errors.Wrap(err, "Failed to unmarshall inline contents")
	}

	// get the tempfile name
	tempFileName := path.Join(os.TempDir(), fileName)

	// write the temporary file
	ioutil.WriteFile(tempFileName, marshalledFileContents, os.FileMode(0644))

	return tempFileName, nil
}

func (b *Builder) pushProcessorImage(processorImageName string) error {
	if b.PushRegistry != "" {
		return b.dockerClient.PushImage(processorImageName, b.PushRegistry)
	}

	return nil
}
