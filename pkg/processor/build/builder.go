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
	"strings"
	"text/template"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/build/inlineparser"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	// load runtimes so that they register to runtime registry
	_ "github.com/nuclio/nuclio/pkg/processor/build/runtime/golang"
	_ "github.com/nuclio/nuclio/pkg/processor/build/runtime/python"
	"github.com/nuclio/nuclio/pkg/processor/build/util"

	"github.com/nuclio/nuclio-sdk"
	"gopkg.in/yaml.v2"
)

const (
	golangRuntimeName      = "golang"
	pythonRuntimeName      = "python"
	functionConfigFileName = "function.yaml"
)

type Builder struct {
	logger nuclio.Logger

	options *platform.BuildOptions

	// the selected runtimg
	runtime runtime.Runtime

	// a temporary directory which contains all the stuff needed to build
	stagingDir string

	// a docker client with which to build stuff
	dockerClient dockerclient.Client

	// inline blocks of configuration, having appeared in the source prefixed with @nuclio.<something>
	inlineConfigurationBlock map[string]interface{}

	// information about the processor image - the one that actually holds the processor binary and is pushed
	// to the cluster
	processorImage struct {

		// a map of local_path:dest_path. each file / dir from local_path will be copied into
		// the docker image at dest_path
		objectsToCopyDuringBuild map[string]string

		// name of the image that will be created
		imageName string

		// the tag of the image that will be created
		imageTag string
	}
}

func NewBuilder(parentLogger nuclio.Logger) (*Builder, error) {
	var err error

	newBuilder := &Builder{
		logger: parentLogger,
	}

	newBuilder.dockerClient, err = dockerclient.NewShellClient(newBuilder.logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create docker client")
	}

	return newBuilder, nil
}

func (b *Builder) Build(options *platform.BuildOptions) (*platform.BuildResult, error) {
	var err error

	b.options = options

	b.logger.InfoWith("Building", "name", b.options.FunctionConfig.Meta.Name)

	// resolve the function path - download in case its a URL
	b.options.FunctionConfig.Spec.Build.Path, err = b.resolveFunctionPath(b.options.FunctionConfig.Spec.Build.Path)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve function path")
	}

	// parse the inline blocks in the file - blocks of comments starting with @nuclio.<something>. this may be used
	// later on (e.g. for creating files)
	if common.IsFile(b.options.FunctionConfig.Spec.Build.Path) {
		b.parseInlineBlocks()
	}

	// prepare configuration from both configuration files and things builder infers
	_, err = b.readConfiguration()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read configuration")
	}

	// create a staging directory
	b.stagingDir, err = b.createStagingDir()
	if err != nil {
		return nil, errors.Wrap(err, "Failed create staging directory")
	}

	// create a runtime based on the configuration
	b.runtime, err = b.createRuntime()
	if err != nil {
		return nil, errors.Wrap(err, "Failed create runtime")
	}

	// once we're done reading our configuration, we may still have to fill in the blanks because
	// since the user isn't obligated to always pass all the configuration
	if err = b.enrichConfiguration(); err != nil {
		return nil, errors.Wrap(err, "Failed to enrich configuration")
	}

	// prepare a staging directory
	if err = b.prepareStagingDir(); err != nil {
		return nil, errors.Wrap(err, "Failed to prepare staging dir")
	}

	// build the processor image
	processorImageName, err := b.buildProcessorImage()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to build processor image")
	}

	// push the processor image
	if err := b.pushProcessorImage(processorImageName); err != nil {
		return nil, errors.Wrap(err, "Failed to push processor image")
	}

	buildResult := &platform.BuildResult{
		ImageName:             processorImageName,
		Runtime:               b.runtime.GetName(),
		Handler:               b.options.FunctionConfig.Spec.Handler,
		UpdatedFunctionConfig: b.options.FunctionConfig,
	}

	b.logger.InfoWith("Build complete", "result", buildResult)

	return buildResult, nil
}

func (b *Builder) GetFunctionPath() string {
	return b.options.FunctionConfig.Spec.Build.Path
}

func (b *Builder) GetFunctionName() string {
	return b.options.FunctionConfig.Meta.Name
}

func (b *Builder) GetFunctionHandler() string {
	return b.options.FunctionConfig.Spec.Handler
}

func (b *Builder) GetNuclioSourceDir() string {
	return b.options.FunctionConfig.Spec.Build.NuclioSourceDir
}

func (b *Builder) GetNuclioSourceURL() string {
	return b.options.FunctionConfig.Spec.Build.NuclioSourceURL
}

func (b *Builder) GetStagingDir() string {
	return b.stagingDir
}

func (b *Builder) GetFunctionDir() string {

	// if the function directory was passed, just return that. if the function path was passed, return the directory
	// the function is in
	if common.IsDir(b.options.FunctionConfig.Spec.Build.Path) {
		return b.options.FunctionConfig.Spec.Build.Path
	}

	return path.Dir(b.options.FunctionConfig.Spec.Build.Path)
}

func (b *Builder) GetNoBaseImagePull() bool {
	return b.options.FunctionConfig.Spec.Build.NoBaseImagesPull
}

func (b *Builder) readConfiguration() (string, error) {

	if functionConfigPath := b.providedFunctionConfigFilePath(); functionConfigPath != "" {
		if err := b.readFunctionConfigFile(functionConfigPath); err != nil {
			return "", errors.Wrap(err, "Failed to read function configuration")
		}

		return functionConfigPath, nil
	}

	return "", nil
}

func (b *Builder) providedFunctionConfigFilePath() string {

	// if the user only provided a function file, check if it had a function configuration file
	// in an inline configuration block (@nuclio.configure)
	if common.IsFile(b.options.FunctionConfig.Spec.Build.Path) {
		inlineFunctionConfig, found := b.inlineConfigurationBlock[functionConfigFileName]
		if !found {
			return ""
		}

		// create a temporary file containing the contents and return that
		functionConfigPath, err := b.createTempFileFromYAML(functionConfigFileName, inlineFunctionConfig)

		b.logger.DebugWith("Function configuration generated from inline", "path", functionConfigPath)

		if err == nil {
			return functionConfigPath
		}
	}

	functionConfigPath := filepath.Join(b.options.FunctionConfig.Spec.Build.Path, functionConfigFileName)

	if !common.FileExists(functionConfigPath) {
		return ""
	}

	b.logger.DebugWith("Function configuration found in directory", "path", functionConfigPath)

	return functionConfigPath
}

func (b *Builder) enrichConfiguration() error {

	// if runtime wasn't passed, use the default from the created runtime
	if b.options.FunctionConfig.Spec.Runtime == "" {
		b.options.FunctionConfig.Spec.Runtime = b.runtime.GetName()
	}

	// if the function handler isn't set, ask runtime
	if b.options.FunctionConfig.Spec.Handler == "" {
		functionHandlers, err := b.runtime.DetectFunctionHandlers(b.GetFunctionPath())
		if err != nil {
			return errors.Wrap(err, "Failed to detect ")
		}

		if len(functionHandlers) == 0 {
			return errors.New("Could not find any handlers")
		}

		// use first for now
		b.options.FunctionConfig.Spec.Handler = functionHandlers[0]
	}

	// if output image name isn't set, set it to a derivative of the name
	if b.processorImage.imageName == "" {
		if b.options.FunctionConfig.Spec.Build.ImageName == "" {
			b.processorImage.imageName = fmt.Sprintf("nuclio/processor-%s", b.GetFunctionName())
		} else {
			b.processorImage.imageName = b.options.FunctionConfig.Spec.Build.ImageName
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
	}

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

func (b *Builder) readFunctionConfigFile(functionConfigPath string) error {

	// read the file once for logging
	functionConfigContents, err := ioutil.ReadFile(functionConfigPath)
	if err != nil {
		return errors.Wrap(err, "Failed to read function configuration file")
	}

	// log
	b.logger.DebugWith("Read function configuration file", "contents", string(functionConfigContents))

	functionConfigFile, err := os.Open(functionConfigPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to open function configuraition file: %s", functionConfigFile)
	}

	defer functionConfigFile.Close()

	functionconfigReader, err := functionconfig.NewReader(b.logger)
	if err != nil {
		return errors.Wrap(err, "Failed to create functionconfig reader")
	}

	// read the configuration
	if err := functionconfigReader.Read(functionConfigFile,
		"yaml",
		&b.options.FunctionConfig); err != nil {

		return errors.Wrap(err, "Failed to read function configuration file")
	}

	return nil
}

func (b *Builder) createRuntime() (runtime.Runtime, error) {
	var err error
	runtimeName := b.options.FunctionConfig.Spec.Runtime

	// if runtime isn't set, try to look at extension
	if runtimeName == "" {

		// if the function path is a directory, assume Go for now
		if common.IsDir(b.options.FunctionConfig.Spec.Build.Path) {
			runtimeName = golangRuntimeName
		} else {
			runtimeName, err = b.getRuntimeNameByFileExtension(b.options.FunctionConfig.Spec.Build.Path)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to get runtime name")
			}
		}

		b.logger.DebugWith("Runtime auto-detected", "runtime", runtimeName)
	}

	// get the first part of the runtime (e.g. go:1.8 -> go)
	runtimeName = strings.Split(runtimeName, ":")[0]

	// if the file extension is of a known runtime, use that
	runtimeFactory, err := runtime.RuntimeRegistrySingleton.Get(runtimeName)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get runtime factory")
	}

	// create a runtime instance
	runtimeInstance, err := runtimeFactory.(runtime.Factory).Create(b.logger,
		b.stagingDir,
		&b.options.FunctionConfig)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create runtime")
	}

	return runtimeInstance, nil
}

func (b *Builder) createStagingDir() (string, error) {

	// create a staging directory
	stagingDir, err := ioutil.TempDir("", "nuclio-build-")
	if err != nil {
		return "", errors.Wrap(err, "Failed to create staging dir")
	}

	b.logger.DebugWith("Created staging directory", "dir", b.stagingDir)

	return stagingDir, nil
}

func (b *Builder) prepareStagingDir() error {

	b.logger.InfoWith("Staging files and preparing base images")

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

	b.logger.DebugWith("Runtime provided objects to staging dir", "objects", objectPathsToStagingDir)

	// add the objects the user requested
	for localObjectPath := range b.options.FunctionConfig.Spec.Build.AddedObjectPaths {
		objectPathsToStagingDir[localObjectPath] = ""
	}

	// copy the files - ignore where we need to copy this in the image, this'll be done later. right now
	// we just want to copy the file from wherever it is to the staging dir root
	for localObjectPath := range objectPathsToStagingDir {

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

	// get the base image name (based on version, base image name, etc)
	baseImageName, err := b.runtime.GetProcessorBaseImageName()
	if err != nil {
		return "", errors.Wrap(err, "Could not find a proper base image for processor")
	}

	processorDockerfileTemplateFuncs := template.FuncMap{
		"pathBase":      path.Base,
		"isDir":         common.IsDir,
		"objectsToCopy": b.getObjectsToCopyToProcessorImage,
		"baseImageName": func() string { return baseImageName },
		"commandsToRun": func() []string { return b.options.FunctionConfig.Spec.Build.Commands },
	}

	processorDockerfileTemplate, err := template.New("").
		Funcs(processorDockerfileTemplateFuncs).
		Parse(processorImageDockerfileTemplate)

	if err != nil {
		return "", errors.Wrap(err, "Failed to parse processor image Dockerfile template")
	}

	processorDockerfilePathInStaging := filepath.Join(b.stagingDir, "Dockerfile.processor")
	processorDockerfileInStaging, err := os.Create(processorDockerfilePathInStaging)
	if err != nil {
		return "", errors.Wrapf(err, "Can't create processor docker file at %s", processorDockerfilePathInStaging)
	}

	b.logger.DebugWith("Creating Dockerfile from template",
		"baseImage", baseImageName,
		"commands", b.options.FunctionConfig.Spec.Build.Commands,
		"dest", processorDockerfilePathInStaging)

	if err = processorDockerfileTemplate.Execute(processorDockerfileInStaging, nil); err != nil {
		return "", errors.Wrapf(err, "Can't execute template")
	}

	return processorDockerfilePathInStaging, nil
}

// returns a map where key is the relative path into staging of a file that needs
// to be copied into the absolute directory in the processor image (the value of that key).
func (b *Builder) getObjectsToCopyToProcessorImage() map[string]string {
	objectsToCopyToProcessorImage := map[string]string{}

	// the runtime specifies key/value where key = absolute local path and
	// value = absolute path into docker. since we already copied these files
	// to the root of staging, we can just take their file name and get relative the
	// path into staging
	for localObjectPath, imageObjectPath := range b.runtime.GetProcessorImageObjectPaths() {
		objectsToCopyToProcessorImage[path.Base(localObjectPath)] = imageObjectPath
	}

	// add the objects the user requested. TODO: support directories
	for localObjectPath, imageObjectPath := range b.options.FunctionConfig.Spec.Build.AddedObjectPaths {
		objectsToCopyToProcessorImage[path.Base(localObjectPath)] = imageObjectPath
	}

	return objectsToCopyToProcessorImage
}

// this will parse the source file looking for @nuclio.configure blocks. It will then generate these files
// in the staging area
func (b *Builder) parseInlineBlocks() error {

	// create an inline block parser
	parser, err := inlineparser.NewParser(b.logger)
	if err != nil {
		return errors.Wrap(err, "Failed to create parser")
	}

	// create a file reader
	functionFile, err := os.OpenFile(b.options.FunctionConfig.Spec.Build.Path, os.O_RDONLY, os.FileMode(0644))
	if err != nil {
		return errors.Wrap(err, "Failed to open function file")
	}

	// get runtime name
	runtimeName, err := b.getRuntimeNameByFileExtension(b.options.FunctionConfig.Spec.Build.Path)
	if err != nil {
		return errors.Wrap(err, "Failed to get runtime name")
	}

	// get comment pattern
	commentPattern, err := b.getRuntimeCommentPattern(runtimeName)
	if err != nil {
		return errors.Wrap(err, "Failed to get runtime comment pattern")
	}

	blocks, err := parser.Parse(functionFile, commentPattern)
	if err != nil {
		return errors.Wrap(err, "Failed to parse inline blocks")
	}

	b.inlineConfigurationBlock = blocks["configure"]

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
	if b.options.FunctionConfig.Spec.Build.Registry != "" {
		return b.dockerClient.PushImage(processorImageName, b.options.FunctionConfig.Spec.Build.Registry)
	}

	return nil
}

func (b *Builder) getRuntimeNameByFileExtension(functionPath string) (string, error) {

	// try to read the file extension (skip dot in extension)
	functionFileExtension := filepath.Ext(functionPath)[1:]

	// if the file extension is of a known runtime, use that (skip dot in extension)
	switch functionFileExtension {
	case "go":
		return golangRuntimeName, nil
	case "py":
		return pythonRuntimeName, nil
	default:
		return "", fmt.Errorf("Unsupported file extension: %s", functionFileExtension)
	}
}

func (b *Builder) getRuntimeCommentPattern(runtimeName string) (string, error) {
	switch runtimeName {
	case golangRuntimeName:
		return "//", nil
	case pythonRuntimeName:
		return "#", nil
	}

	return "", fmt.Errorf("Unsupported runtime name: %s", runtimeName)
}
