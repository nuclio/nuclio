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
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
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
	_ "github.com/nuclio/nuclio/pkg/processor/build/runtime/dotnetcore"
	_ "github.com/nuclio/nuclio/pkg/processor/build/runtime/golang"
	_ "github.com/nuclio/nuclio/pkg/processor/build/runtime/java"
	_ "github.com/nuclio/nuclio/pkg/processor/build/runtime/nodejs"
	//_ "github.com/nuclio/nuclio/pkg/processor/build/runtime/pypy"
	_ "github.com/nuclio/nuclio/pkg/processor/build/runtime/python"
	_ "github.com/nuclio/nuclio/pkg/processor/build/runtime/ruby"
	_ "github.com/nuclio/nuclio/pkg/processor/build/runtime/shell"
	"github.com/nuclio/nuclio/pkg/processor/build/util"
	"github.com/nuclio/nuclio/pkg/version"

	"github.com/nuclio/logger"
	"github.com/rs/xid"
	"gopkg.in/yaml.v2"
)

const (
	functionConfigFileName = "function.yaml"
	uhttpcImage            = "quay.io/nuclio/uhttpc:0.0.1-amd64"
	GithubEntryType        = "github"
	ArchiveEntryType       = "archive"
	S3EntryType            = "s3"
)

// holds parameters for things that are required before a runtime can be initialized
type runtimeInfo struct {
	extension    string
	inlineParser inlineparser.ConfigParser

	// used to prioritize runtimes, like when there is more than one runtime matching a given criteria (e.g.
	// pypy and python have the same extension)
	weight int
}

// Builder builds user handlers
type Builder struct {
	logger logger.Logger

	platform platform.Platform

	options *platform.CreateFunctionBuildOptions

	// the selected runtime
	runtime runtime.Runtime

	// a temporary directory which contains all the stuff needed to build
	tempDir string

	// full path to staging directory (under tempDir) which is used as the docker build context for the function
	stagingDir string

	// a docker client with which to build stuff
	dockerClient dockerclient.Client

	// inline blocks of configuration, having appeared in the source prefixed with @nuclio.<something>
	inlineConfigurationBlock inlineparser.Block

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

	// a map of support runtimes
	runtimeInfo map[string]runtimeInfo

	// original function configuration, for fields that are overridden and need to be restored
	originalFunctionConfig functionconfig.Config

	s3Client common.S3Client
}

// NewBuilder returns a new builder
func NewBuilder(parentLogger logger.Logger, platform platform.Platform, s3Client common.S3Client) (*Builder, error) {
	var err error

	newBuilder := &Builder{
		logger:   parentLogger,
		platform: platform,
		s3Client: s3Client,
	}

	newBuilder.initializeSupportedRuntimes()

	newBuilder.dockerClient, err = dockerclient.NewShellClient(newBuilder.logger, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create docker client")
	}

	return newBuilder, nil
}

// Build builds the handler
func (b *Builder) Build(options *platform.CreateFunctionBuildOptions) (*platform.CreateFunctionBuildResult, error) {
	var err error

	b.options = options

	b.logger.InfoWith("Building", "name", b.options.FunctionConfig.Meta.Name)

	configurationRead := false
	configFilePath := b.providedFunctionConfigFilePath()
	b.logger.DebugWith("Function configuration found in directory", "configFilePath", configFilePath)
	if common.IsFile(configFilePath) {
		if _, err = b.readConfiguration(); err != nil {
			return nil, errors.Wrap(err, "Failed to read configuration")
		}
		configurationRead = true
	}

	// create base temp directory
	err = b.createTempDir()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create base temp dir")
	}

	defer b.cleanupTempDir() // nolint: errcheck

	// create staging directory
	err = b.createStagingDir()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create staging dir")
	}

	// before we resolve the path, save it so that we can restore it later
	b.originalFunctionConfig.Spec.Build.Path = b.options.FunctionConfig.Spec.Build.Path

	// resolve the function path - download in case its a URL
	b.options.FunctionConfig.Spec.Build.Path, err = b.resolveFunctionPath(b.options.FunctionConfig.Spec.Build.Path)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve function path")
	}

	// parse the inline blocks in the file - blocks of comments starting with @nuclio.<something>. this may be used
	// later on (e.g. for creating files)
	if common.IsFile(b.options.FunctionConfig.Spec.Build.Path) {
		var functionSourceCode string

		// see if there are any inline blocks in the code. ignore errors during parse / load / whatever
		b.parseInlineBlocks() // nolint: errcheck

		// dont fail on parseInlineBlocks so that if the parser fails on something we won't block deployments. the only
		// exception is if the user provided a block with improper contents
		if b.inlineConfigurationBlock.Error != nil {
			return nil, errors.Wrap(b.inlineConfigurationBlock.Error, "Failed to parse inline configuration")
		}

		// try to see if we need to convert the file path -> functionSourceCode
		functionSourceCode, err = b.getSourceCodeFromFilePath()
		if err != nil {
			b.logger.DebugWith("Not populating function source code", "reason", errors.Cause(err))
		} else {

			// set into source code
			b.logger.DebugWith("Populating functionSourceCode from file path", "contents", functionSourceCode)
			b.options.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString([]byte(functionSourceCode))
		}
	}

	// prepare configuration from both configuration files and things builder infers
	if !configurationRead {
		_, err = b.readConfiguration()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to read configuration")
		}
	}

	// create a runtime based on the configuration
	b.runtime, err = b.createRuntime()
	if err != nil {
		return nil, errors.Wrap(err, "Failed create runtime")
	}

	// once we're done reading our configuration, we may still have to fill in the blanks
	// because the user isn't obligated to always pass all the configuration
	if err = b.validateAndEnrichConfiguration(); err != nil {
		return nil, errors.Wrap(err, "Failed to enrich configuration")
	}

	// copy the configuration we enriched, restoring any fields that should not be leaked externally
	enrichedConfiguration := b.options.FunctionConfig
	enrichedConfiguration.Spec.Build.Path = b.originalFunctionConfig.Spec.Build.Path

	// if a callback is registered, call back
	if b.options.OnAfterConfigUpdate != nil {
		if err = b.options.OnAfterConfigUpdate(&enrichedConfiguration); err != nil {
			return nil, errors.Wrap(err, "OnAfterConfigUpdate returned error")
		}
	}

	// prepare a staging directory
	if err = b.prepareStagingDir(); err != nil {
		return nil, errors.Wrap(err, "Failed to prepare staging dir")
	}

	// build the processor image
	processorImage, err := b.buildProcessorImage()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to build processor image")
	}

	// push the processor image
	if err := b.pushProcessorImage(processorImage); err != nil {
		return nil, errors.Wrap(err, "Failed to push processor image")
	}

	buildResult := &platform.CreateFunctionBuildResult{
		Image:                 processorImage,
		UpdatedFunctionConfig: enrichedConfiguration,
	}

	b.logger.InfoWith("Build complete", "result", buildResult)

	if b.options.OutputImageFile != "" {
		b.logger.InfoWith("Saving built docker image as archive", "outputFile", b.options.OutputImageFile)
		err := b.dockerClient.Save(processorImage, b.options.OutputImageFile)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to save docker image")
		}
	}

	return buildResult, nil
}

// GetFunctionPath returns the path to the function
func (b *Builder) GetFunctionPath() string {
	return b.options.FunctionConfig.Spec.Build.Path
}

// GetFunctionName returns the name of the function
func (b *Builder) GetFunctionName() string {
	return b.options.FunctionConfig.Meta.Name
}

// GetFunctionHandler returns the name of the handler
func (b *Builder) GetFunctionHandler() string {
	return b.options.FunctionConfig.Spec.Handler
}

// GetStagingDir returns path to the staging directory
func (b *Builder) GetStagingDir() string {
	return b.stagingDir
}

// GetFunctionDir return path to function directory inside the staging directory
func (b *Builder) GetFunctionDir() string {

	// if the function directory was passed, just return that. if the function path was passed, return the directory
	// the function is in
	if common.IsDir(b.options.FunctionConfig.Spec.Build.Path) {
		return b.options.FunctionConfig.Spec.Build.Path
	}

	return path.Dir(b.options.FunctionConfig.Spec.Build.Path)
}

// GetNoBaseImagePull return true if we shouldn't pull base images
func (b *Builder) GetNoBaseImagePull() bool {
	return b.options.FunctionConfig.Spec.Build.NoBaseImagesPull
}

func (b *Builder) initializeSupportedRuntimes() {
	b.runtimeInfo = map[string]runtimeInfo{}

	// create a few shared parsers
	slashSlashParser := inlineparser.NewParser(b.logger, "//")
	poundParser := inlineparser.NewParser(b.logger, "#")

	b.runtimeInfo["shell"] = runtimeInfo{"sh", poundParser, 0}
	b.runtimeInfo["golang"] = runtimeInfo{"go", slashSlashParser, 0}
	b.runtimeInfo["pypy"] = runtimeInfo{"py", poundParser, 0}
	b.runtimeInfo["python"] = runtimeInfo{"py", poundParser, 10}
	b.runtimeInfo["python:2.7"] = runtimeInfo{"py", poundParser, 5}
	b.runtimeInfo["python:3.6"] = runtimeInfo{"py", poundParser, 5}
	b.runtimeInfo["nodejs"] = runtimeInfo{"js", slashSlashParser, 0}
	b.runtimeInfo["java"] = runtimeInfo{"java", slashSlashParser, 0}
	b.runtimeInfo["ruby"] = runtimeInfo{"rb", poundParser, 0}
	b.runtimeInfo["dotnetcore"] = runtimeInfo{"cs", slashSlashParser, 0}
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

	// if the user provided a configuration path, use that
	if b.options.FunctionConfig.Spec.Build.FunctionConfigPath != "" {
		return b.options.FunctionConfig.Spec.Build.FunctionConfigPath
	}

	// if the user only provided a function file, check if it had a function configuration file
	// in an inline configuration block (@nuclio.configure)
	if common.IsFile(b.options.FunctionConfig.Spec.Build.Path) {
		inlineFunctionConfig, found := b.inlineConfigurationBlock.Contents[functionConfigFileName]
		if !found {
			return ""
		}

		// create a temporary file containing the contents and return that
		functionConfigPath, err := b.createTempFileFromYAML(functionConfigFileName, inlineFunctionConfig)

		b.logger.DebugWith("Function configuration generated from inline", "path", functionConfigPath)

		if err == nil {
			return functionConfigPath
		}

		b.logger.WarnWith("Failed to unmarshal inline configuration - ignoring", "err", err)
	}

	functionConfigPath := filepath.Join(b.options.FunctionConfig.Spec.Build.Path, functionConfigFileName)

	if !common.FileExists(functionConfigPath) {
		return ""
	}

	return functionConfigPath
}

func (b *Builder) validateAndEnrichConfiguration() error {
	if b.options.FunctionConfig.Meta.Name == "" {
		return errors.New("Function must have a name")
	}

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
		b.processorImage.imageName = b.getImage()
	}

	// if tag isn't set - set latest
	if b.processorImage.imageTag == "" {
		b.processorImage.imageTag = "latest"
	}

	b.logger.DebugWith("Enriched configuration", "options", b.options, "pi", b.processorImage)

	return nil
}

func (b *Builder) getImage() string {
	var imageName string

	if b.options.FunctionConfig.Spec.Build.Image == "" {
		repository := "nuclio/"

		// try to see if the registry URL has a repository specified (e.g. localhost:5000/foo). If so,
		// don't use "nuclio/", just use that repository
		parsedRegistryURL, err := url.Parse(b.options.FunctionConfig.Spec.Build.Registry)
		if err == nil {
			if len(parsedRegistryURL.Path) > 0 {
				repository = ""
			}
		}

		imageName = fmt.Sprintf("%sprocessor-%s", repository, b.GetFunctionName())
	} else {
		imageName = b.options.FunctionConfig.Spec.Build.Image
	}

	return imageName
}

func (b *Builder) writeFunctionSourceCodeToTempFile(functionSourceCode string) (string, error) {
	if b.options.FunctionConfig.Spec.Runtime == "" {
		return "", errors.New("Runtime must be explicitly defined when using Function Source Code")
	}

	// prepare a slice with enough underlying space
	decodedFunctionSourceCode, err := base64.StdEncoding.DecodeString(functionSourceCode)
	if err != nil {
		return "", errors.Wrap(err, "Failed to decode function source code")
	}

	tempDir, err := b.mkDirUnderTemp("source")
	if err != nil {
		return "", errors.Wrapf(err, "Failed to create temporary dir for function code: %s", tempDir)
	}

	runtimeExtension, err := b.getRuntimeFileExtensionByName(b.options.FunctionConfig.Spec.Runtime)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to get file extension for runtime %s", b.options.FunctionConfig.Spec.Runtime)
	}

	// we will generate a file named as per specified by the handler
	moduleFileName, entrypoint, err := functionconfig.ParseHandler(b.options.FunctionConfig.Spec.Handler)
	if err != nil {
		return "", errors.Wrap(err, "Failed to parse handler")
	}

	if moduleFileName == "" {
		moduleFileName = entrypoint
	}

	// if the module name already an extension, leave it
	if !strings.Contains(moduleFileName, ".") {
		moduleFileName = fmt.Sprintf("%s.%s", moduleFileName, runtimeExtension)
	}

	sourceFilePath := path.Join(tempDir, moduleFileName)

	b.logger.DebugWith("Writing function source code to temporary file", "functionPath", sourceFilePath)
	err = ioutil.WriteFile(sourceFilePath, decodedFunctionSourceCode, os.FileMode(0644))
	if err != nil {
		return "", errors.Wrapf(err, "Failed to write given source code to file %s", sourceFilePath)
	}

	return sourceFilePath, nil
}

func (b *Builder) resolveFunctionPath(functionPath string) (string, error) {
	var err error

	if b.options.FunctionConfig.Spec.Build.FunctionSourceCode != "" {

		// if user gave function as source code rather than a path - write it to a temporary file
		functionSourceCodeTempPath, err := b.writeFunctionSourceCodeToTempFile(b.options.FunctionConfig.Spec.Build.FunctionSourceCode)
		if err != nil {
			return "", errors.Wrap(err, "Failed to save function code to temporary file")
		}

		return functionSourceCodeTempPath, nil
	}

	codeEntryType := b.options.FunctionConfig.Spec.Build.CodeEntryType

	// function can either be in the path, received inline or an executable via handler
	if functionPath == "" &&
		b.options.FunctionConfig.Spec.Image == "" &&
		codeEntryType != S3EntryType {

		if b.options.FunctionConfig.Spec.Runtime != "shell" {
			return "", errors.New("Function path must be provided when specified runtime isn't shell")
		}

		// did user give handler to an executable
		if b.options.FunctionConfig.Spec.Handler == "" {
			return "", errors.New("If shell runtime is specified, function path or handler name must be provided")
		}
	}

	// user has to provide valid url when code entry type is github
	if !common.IsURL(functionPath) && codeEntryType == GithubEntryType {
		return "", errors.New("Must provide valid URL when code entry type is github or archive")
	}

	// if the function path is a URL, type is Github or S3 - first download the file
	// for backwards compatibility, don't check for entry type url specifically
	if functionPath, err = b.resolveFunctionPathFromURL(functionPath, codeEntryType); err != nil {
		return "", errors.Wrap(err, "Failed to download function from URL")
	}

	// Assume it's a local path
	resolvedPath, err := filepath.Abs(filepath.Clean(functionPath))
	if err != nil {
		return "", errors.Wrap(err, "Failed to get resolve non-url path")
	}

	if !common.FileExists(resolvedPath) {
		return "", fmt.Errorf("Function path doesn't exist: %s", resolvedPath)
	}

	if util.IsCompressed(resolvedPath) {
		resolvedPath, err = b.decompressFunctionArchive(resolvedPath)
		if err != nil {
			return "", errors.Wrap(err, "Failed to decompress function archive")
		}
	}

	return resolvedPath, nil
}

func (b *Builder) validateAndParseS3Attributes(attributes map[string]interface{}) (map[string]string, error) {
	parsedAttributes := map[string]string{}

	mandatoryFields := []string{"s3Bucket", "s3ItemKey"}
	optionalFields := []string{"s3Region", "s3AccessKeyId", "s3SecretAccessKey", "s3SessionToken"}

	for _, key := range append(mandatoryFields, optionalFields...) {
		value, found := attributes[key]
		if !found {
			if common.StringInSlice(key, mandatoryFields) {
				return nil, fmt.Errorf("Mandatory field - '%s' not given", key)
			}
			continue
		}
		valueAsString, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("The given field - '%s' is not of type string", key)
		}
		parsedAttributes[key] = valueAsString
	}

	return parsedAttributes, nil
}

func (b *Builder) getFunctionPathFromGithubURL(functionPath string) (string, error) {
	if branch, ok := b.options.FunctionConfig.Spec.Build.CodeEntryAttributes["branch"]; ok {
		functionPath = fmt.Sprintf("%s/archive/%s.zip",
			strings.TrimRight(functionPath, "/"),
			branch)
	} else {
		return "", errors.New("If code entry type is github, branch must be provided")
	}
	return functionPath, nil
}

func (b *Builder) decompressFunctionArchive(functionPath string) (string, error) {

	// create a staging directory
	decompressDir, err := b.mkDirUnderTemp("decompress")
	if err != nil {
		return "", errors.Wrapf(err, "Failed to create temporary directory for decompressing archive %v", functionPath)
	}

	decompressor, err := util.NewDecompressor(b.logger)
	if err != nil {
		return "", errors.Wrap(err, "Failed to instantiate decompressor")
	}

	err = decompressor.Decompress(functionPath, decompressDir)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to decompress file %s", functionPath)
	}

	codeEntryType := b.options.FunctionConfig.Spec.Build.CodeEntryType
	if codeEntryType == GithubEntryType {
		decompressDir, err = b.resolveGithubArchiveWorkDir(decompressDir)
		if err != nil {
			return "", errors.Wrap(err, "Failed to get decompressed directory of entry type github")
		}
	}

	return b.resolveUserSpecifiedArchiveWorkdir(decompressDir)
}

func (b *Builder) resolveGithubArchiveWorkDir(decompressDir string) (string, error) {
	directories, err := ioutil.ReadDir(decompressDir)
	if err != nil {
		return "", errors.Wrap(err, "Failed to list decompressed directory tree")
	}

	// when code entry type is github assume only one directory under root
	directory := directories[0]

	if directory.IsDir() {
		decompressDir = filepath.Join(decompressDir, directory.Name())
	} else {
		return "", errors.New("Unexpected non directory found with entry code type github")
	}

	return decompressDir, nil
}

func (b *Builder) resolveUserSpecifiedArchiveWorkdir(decompressDir string) (string, error) {
	codeEntryType := b.options.FunctionConfig.Spec.Build.CodeEntryType
	userSpecifiedWorkDirectoryInterface, found := b.options.FunctionConfig.Spec.Build.CodeEntryAttributes["workDir"]

	if (codeEntryType == ArchiveEntryType || codeEntryType == GithubEntryType || codeEntryType == S3EntryType) && found {
		userSpecifiedWorkDirectory, ok := userSpecifiedWorkDirectoryInterface.(string)
		if !ok {
			return "", errors.New("If code entry type is (archive or github) and workDir is provided, " +
				"workDir expected to be string")
		}
		decompressDir = filepath.Join(decompressDir, userSpecifiedWorkDirectory)
	}
	return decompressDir, nil
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
		return errors.Wrapf(err, "Failed to open function configuraition file: %q", functionConfigFile.Name())
	}

	defer functionConfigFile.Close() // nolint: errcheck

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
	runtimeName, err := b.getRuntimeName()

	if err != nil {
		return nil, err
	}

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

func (b *Builder) getRuntimeName() (string, error) {
	var err error
	runtimeName := b.options.FunctionConfig.Spec.Runtime

	// if runtime isn't set, try to look at extension
	if runtimeName == "" {

		// if the function path is a directory, runtime must be specified in the command-line arguments or configuration
		if common.IsDir(b.options.FunctionConfig.Spec.Build.Path) {
			if common.FileExists(path.Join(b.options.FunctionConfig.Spec.Build.Path, functionConfigFileName)) {
				return "", errors.New("Build path is directory - function.yaml must specify runtime")
			}

			return "", errors.New("Build path is directory - runtime must be specified")
		}

		runtimeName, err = b.getRuntimeNameByFileExtension(b.options.FunctionConfig.Spec.Build.Path)
		if err != nil {
			return "", errors.Wrap(err, "Failed to get runtime name")
		}

		b.logger.DebugWith("Runtime auto-detected", "runtime", runtimeName)
	}

	// get the first part of the runtime (e.g. go:1.8 -> go)
	runtimeName = strings.Split(runtimeName, ":")[0]

	return runtimeName, nil
}

func (b *Builder) createTempDir() error {
	var err error

	// either use injected temporary dir or generate a new one
	if b.options.FunctionConfig.Spec.Build.TempDir != "" {
		b.tempDir = b.options.FunctionConfig.Spec.Build.TempDir

		err = os.MkdirAll(b.tempDir, 0744)

	} else {
		b.tempDir, err = ioutil.TempDir("", "nuclio-build-")
	}

	if err != nil {
		return errors.Wrapf(err, "Failed to create temporary dir %s", b.tempDir)
	}

	b.logger.DebugWith("Created base temporary dir", "dir", b.tempDir)

	return nil
}

func (b *Builder) createStagingDir() error {
	var err error

	b.stagingDir, err = b.mkDirUnderTemp("staging")
	if err != nil {
		return errors.Wrapf(err, "Failed to create staging dir: %s", b.stagingDir)
	}

	b.logger.DebugWith("Created staging dir", "dir", b.stagingDir)

	return nil
}

func (b *Builder) prepareStagingDir() error {
	b.logger.InfoWith("Staging files and preparing base images")

	handlerDirInStaging := b.getHandlerDir(b.stagingDir)

	// make sure the handler stagind dir exists
	if err := os.MkdirAll(handlerDirInStaging, 0755); err != nil {
		return errors.Wrapf(err, "Failed to create handler path in staging @ %s", handlerDirInStaging)
	}

	// first, tell the specific runtime to do its thing
	if err := b.runtime.OnAfterStagingDirCreated(b.stagingDir); err != nil {
		return errors.Wrap(err, "Failed to prepare staging dir")
	}

	// copy any objects the runtime needs into staging
	if err := b.copyHandlerToStagingDir(); err != nil {
		return errors.Wrap(err, "Failed to prepare staging dir")
	}

	return nil
}

func (b *Builder) copyHandlerToStagingDir() error {
	handlerDirObjectPaths := b.runtime.GetHandlerDirObjectPaths()
	handlerDirInStaging := b.getHandlerDir(b.stagingDir)

	b.logger.DebugWith("Runtime provided handler objects to staging dir",
		"handlerDirObjectPaths", handlerDirObjectPaths,
		"handlerDirInStaging", handlerDirInStaging)

	// copy the files - ignore where we need to copy this in the image, this'll be done later. right now
	// we just want to copy the file from wherever it is to the staging dir root
	for _, handlerDirObjectPath := range handlerDirObjectPaths {

		// copy the object (TODO: most likely will need to better support dirs)
		if err := util.CopyTo(handlerDirObjectPath, handlerDirInStaging); err != nil {
			return errors.Wrap(err, "Failed to copy handler object")
		}
	}

	return nil
}

func (b *Builder) mkDirUnderTemp(name string) (string, error) {

	dir := path.Join(b.tempDir, name)

	// temp dir needs executable permission for docker to be able to pull from it
	err := os.Mkdir(dir, 0744)

	if err != nil {
		return "", errors.Wrapf(err, "Failed to create temporary subdir %s", dir)
	}

	b.logger.DebugWith("Created temporary dir", "dir", dir)

	return dir, nil
}

func (b *Builder) cleanupTempDir() error {
	if b.options.FunctionConfig.Spec.Build.NoCleanup {
		b.logger.Debug("no-cleanup flag provided, skipping temporary dir cleanup")
		return nil
	}

	err := os.RemoveAll(b.tempDir)
	if err != nil {
		return errors.Wrapf(err, "Failed to clean up temporary dir %s", b.tempDir)
	}

	b.logger.DebugWith("Successfully cleaned up temporary dir",
		"dir", b.tempDir)
	return nil
}

func (b *Builder) buildProcessorImage() (string, error) {
	buildArgs, err := b.getBuildArgs()
	if err != nil {
		return "", errors.Wrap(err, "Failed to get build args")
	}

	processorDockerfilePathInStaging, err := b.createProcessorDockerfile()
	if err != nil {
		return "", errors.Wrap(err, "Failed to create processor dockerfile")
	}

	imageName := fmt.Sprintf("%s:%s", b.processorImage.imageName, b.processorImage.imageTag)

	b.logger.InfoWith("Building processor image", "imageName", imageName)

	err = b.dockerClient.Build(&dockerclient.BuildOptions{
		ContextDir:     b.stagingDir,
		Image:          imageName,
		DockerfilePath: processorDockerfilePathInStaging,
		NoCache:        b.options.FunctionConfig.Spec.Build.NoCache,
		BuildArgs:      buildArgs,
	})

	return imageName, err
}

func (b *Builder) createProcessorDockerfile() (string, error) {

	// get the contents of the processor dockerfile from the runtime
	processorDockerfileContents, err := b.getRuntimeProcessorDockerfileContents()
	if err != nil {
		return "", errors.Wrap(err, "Failed to get Dockerfile contents")
	}

	// generated dockerfile should reside in staging
	processorDockerfilePathInStaging := filepath.Join(b.stagingDir, "Dockerfile.processor")

	// log the resulting dockerfile
	b.logger.DebugWith("Created processor Dockerfile", "contents", processorDockerfileContents)

	// write the contents to the path
	if err := ioutil.WriteFile(processorDockerfilePathInStaging,
		[]byte(processorDockerfileContents),
		0644); err != nil {
		return "", errors.Wrap(err, "Failed to write processor Dockerfile")
	}

	return processorDockerfilePathInStaging, nil
}

// this will parse the source file looking for @nuclio.configure blocks. It will then generate these files
// in the staging area
func (b *Builder) parseInlineBlocks() error {

	// get runtime name
	runtimeName, err := b.getRuntimeNameByFileExtension(b.options.FunctionConfig.Spec.Build.Path)
	if err != nil {
		return errors.Wrap(err, "Failed to get runtime name")
	}

	// get comment pattern
	commentParser, err := b.getRuntimeCommentParser(b.logger, runtimeName)
	if err != nil {
		return errors.Wrap(err, "Failed to get runtime comment parser")
	}

	blocks, err := commentParser.Parse(b.options.FunctionConfig.Spec.Build.Path)
	if err != nil {
		return errors.Wrap(err, "Failed to parse inline blocks")
	}

	b.inlineConfigurationBlock = blocks["configure"]

	b.logger.DebugWith("Parsed inline blocks",
		"rawContents",
		b.inlineConfigurationBlock.RawContents,
		"err",
		b.inlineConfigurationBlock.Error)

	return nil
}

// create a temporary file from an unmarshalled YAML
func (b *Builder) createTempFileFromYAML(fileName string, unmarshalledYAMLContents interface{}) (string, error) {
	marshalledFileContents, err := yaml.Marshal(unmarshalledYAMLContents)
	if err != nil {
		return "", errors.Wrap(err, "Failed to unmarshal inline contents")
	}

	// get the tempfile name
	tempFileName := path.Join(os.TempDir(), fileName)

	// write the temporary file
	if err := ioutil.WriteFile(tempFileName, marshalledFileContents, os.FileMode(0744)); err != nil {
		return "", errors.Wrap(err, "Failed to write temporary file")
	}

	return tempFileName, nil
}

func (b *Builder) pushProcessorImage(processorImage string) error {
	if b.options.FunctionConfig.Spec.Build.Registry != "" {
		return b.dockerClient.PushImage(processorImage, b.options.FunctionConfig.Spec.Build.Registry)
	}

	return nil
}

func (b *Builder) getRuntimeNameByFileExtension(functionPath string) (string, error) {

	// try to read the file extension
	functionFileExtension := filepath.Ext(functionPath)
	if functionFileExtension == "" {
		return "", fmt.Errorf("Filepath %s has no extension", functionPath)
	}

	// Remove the final period
	functionFileExtension = functionFileExtension[1:]

	var candidateRuntimeName string

	// iterate over runtime information and return the name by extension
	for runtimeName, runtimeInfo := range b.runtimeInfo {
		if runtimeInfo.extension == functionFileExtension {

			// if there's no candidate yet
			// or if there was a previous candidate with lower weight
			// set current runtime as the candidate
			if candidateRuntimeName == "" || (b.runtimeInfo[candidateRuntimeName].weight < runtimeInfo.weight) {

				// set candidate name
				candidateRuntimeName = runtimeName
			}
		}
	}

	if candidateRuntimeName == "" {
		return "", fmt.Errorf("Unsupported file extension: %s", functionFileExtension)
	}

	return candidateRuntimeName, nil
}

func (b *Builder) getRuntimeFileExtensionByName(runtimeName string) (string, error) {
	runtimeInfo, found := b.runtimeInfo[runtimeName]
	if !found {
		return "", fmt.Errorf("Unsupported runtime name: %s", runtimeName)
	}

	return runtimeInfo.extension, nil
}

func (b *Builder) getRuntimeCommentParser(logger logger.Logger, runtimeName string) (inlineparser.ConfigParser, error) {
	runtimeInfo, found := b.runtimeInfo[runtimeName]
	if !found {
		return nil, fmt.Errorf("Unsupported runtime name: %s", runtimeName)
	}

	return runtimeInfo.inlineParser, nil
}

func (b *Builder) getHandlerDir(stagingDir string) string {
	return path.Join(stagingDir, "handler")
}

func (b *Builder) getRuntimeProcessorDockerfileContents() (string, error) {

	// gather the processor dockerfile info
	processorDockerfileInfo, err := b.getProcessorDockerfileInfo()
	if err != nil {
		return "", errors.Wrap(err, "Failed to get processor Dockerfile info")
	}

	// gather artifacts
	artifactDirNameInStaging := "artifacts"
	artifactsDir := path.Join(b.stagingDir, artifactDirNameInStaging)

	err = b.gatherArtifactsForSingleStageDockerfile(artifactsDir, processorDockerfileInfo)
	if err != nil {
		return "", errors.Wrap(err, "Failed to gather artifacts for single stage Dockerfile")
	}

	// get directives
	directives := b.options.FunctionConfig.Spec.Build.Directives

	// convert commands to directives if commands have values in them (backwards compatibility)
	if len(b.options.FunctionConfig.Spec.Build.Commands) != 0 {
		directives, err = b.commandsToDirectives(b.options.FunctionConfig.Spec.Build.Commands)

		if err != nil {
			return "", errors.Wrap(err, "Failed to convert commands to directives")
		}
	}

	// merge directives passed by user with directives passed by runtime
	directives = b.mergeDirectives(directives, processorDockerfileInfo.Directives)

	// generate single stage dockerfile contents
	return b.generateSingleStageDockerfileContents(artifactDirNameInStaging,
		processorDockerfileInfo.BaseImage,
		processorDockerfileInfo.OnbuildArtifactPaths,
		processorDockerfileInfo.ImageArtifactPaths,
		directives,
		b.platform.GetHealthCheckMode() == platform.HealthCheckModeInternalClient)
}

func (b *Builder) getProcessorDockerfileInfo() (*runtime.ProcessorDockerfileInfo, error) {
	versionInfo, err := version.Get()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get version info")
	}

	// get defaults from the runtime
	runtimeProcessorDockerfileInfo, err := b.runtime.GetProcessorDockerfileInfo(versionInfo)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get processor Dockerfile info")
	}

	processorDockerfileInfo := runtime.ProcessorDockerfileInfo{
		OnbuildArtifactPaths: runtimeProcessorDockerfileInfo.OnbuildArtifactPaths,
		ImageArtifactPaths:   runtimeProcessorDockerfileInfo.ImageArtifactPaths,
		Directives:           runtimeProcessorDockerfileInfo.Directives,
	}

	// set the base image
	processorDockerfileInfo.BaseImage = b.getProcessorDockerfileBaseImage(runtimeProcessorDockerfileInfo.BaseImage)

	// set the onbuild image
	processorDockerfileInfo.OnbuildImage, err = b.getProcessorDockerfileOnbuildImage(versionInfo,
		runtimeProcessorDockerfileInfo.OnbuildImage)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get onbuild image")
	}

	processorDockerfileInfo.BaseImage, err = b.renderDependantImageURL(processorDockerfileInfo.BaseImage,
		b.options.DependantImagesRegistryURL)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to render base image")
	}

	processorDockerfileInfo.OnbuildImage, err = b.renderDependantImageURL(processorDockerfileInfo.OnbuildImage,
		b.options.DependantImagesRegistryURL)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to render onbuild image")
	}

	return &processorDockerfileInfo, nil
}

func (b *Builder) getProcessorDockerfileBaseImage(runtimeDefaultBaseImage string) string {

	// override base image, if required
	switch b.options.FunctionConfig.Spec.Build.BaseImage {

	// if user didn't pass anything, use default as specified in Dockerfile
	case "":
		return runtimeDefaultBaseImage

	// if user specified something - use that
	default:
		return b.options.FunctionConfig.Spec.Build.BaseImage
	}
}

func (b *Builder) getProcessorDockerfileOnbuildImage(versionInfo *version.Info,
	runtimeDefaultOnbuildImage string) (string, error) {

	// if the user supplied an onbuild image, format it with the appropriate tag,
	if b.options.FunctionConfig.Spec.Build.OnbuildImage != "" {
		onbuildImageTemplate, err := template.New("onbuildImage").Parse(b.options.FunctionConfig.Spec.Build.OnbuildImage)
		if err != nil {
			return "", errors.Wrap(err, "Failed to create onbuildImage template")
		}

		var onbuildImageTemplateBuffer bytes.Buffer
		err = onbuildImageTemplate.Execute(&onbuildImageTemplateBuffer, &map[string]interface{}{
			"Label": versionInfo.Label,
			"Arch":  versionInfo.Arch,
		})

		if err != nil {
			return "", errors.Wrap(err, "Failed to run template")
		}

		onbuildImage := onbuildImageTemplateBuffer.String()

		b.options.Logger.DebugWith("Using user provided onbuild image",
			"onbuildImageTemplate", b.options.FunctionConfig.Spec.Build.OnbuildImage,
			"onbuildImage", onbuildImage)

		return onbuildImage, nil
	}

	return runtimeDefaultOnbuildImage, nil
}

func (b *Builder) generateSingleStageDockerfileContents(artifactDirNameInStaging string,
	baseImage string,
	onbuildArtifactPaths map[string]string,
	imageArtifactPaths map[string]string,
	directives map[string][]functionconfig.Directive,
	healthCheckRequired bool) (string, error) {

	// now that all artifacts are in the artifacts directory, we can craft a single stage Dockerfile
	dockerfileTemplateContents := `# From the base image
FROM {{ .BaseImage }}

# Old(er) Docker support - must use all build args
ARG NUCLIO_LABEL
ARG NUCLIO_ARCH
ARG NUCLIO_BUILD_LOCAL_HANDLER_DIR

{{ if .PreCopyDirectives }}
# Run the pre-copy directives
{{ range $directive := .PreCopyDirectives }}
{{ $directive.Kind }} {{ $directive.Value }}
{{ end }}
{{ end }}

{{ if .HealthcheckRequired }}
# Copy health checker
COPY artifacts/uhttpc /usr/local/bin/uhttpc

# Readiness probe
HEALTHCHECK --interval=1s --timeout=3s CMD /usr/local/bin/uhttpc --url http://127.0.0.1:8082/ready || exit 1
{{ end }}

# Copy required objects from the suppliers
{{ range $localArtifactPath, $imageArtifactPath := .OnbuildArtifactPaths }}
COPY {{ $localArtifactPath }} {{ $imageArtifactPath }}
{{ end }}

{{ range $localArtifactPath, $imageArtifactPath := .ImageArtifactPaths }}
COPY {{ $localArtifactPath }} {{ $imageArtifactPath }}
{{ end }}

# Run the post-copy directives
{{ range $directive := .PostCopyDirectives }}
{{ $directive.Kind }} {{ $directive.Value }}
{{ end }}

# Run processor with configuration and platform configuration
CMD [ "processor" ]
`

	// maps between a _relative_ path in staging to the path in the image
	relativeOnbuildArtifactPaths := map[string]string{}
	for localArtifactPath, imageArtifactPath := range onbuildArtifactPaths {
		relativeArtifactPathInStaging := path.Join(artifactDirNameInStaging, path.Base(localArtifactPath))

		relativeOnbuildArtifactPaths[relativeArtifactPathInStaging] = imageArtifactPath
	}

	dockerfileTemplate, err := template.New("singleStageDockerfile").
		Parse(dockerfileTemplateContents)
	if err != nil {
		return "", errors.Wrap(err, "Failed to create onbuildImage template")
	}

	var dockerfileTemplateBuffer bytes.Buffer
	err = dockerfileTemplate.Execute(&dockerfileTemplateBuffer, &map[string]interface{}{
		"BaseImage":            baseImage,
		"OnbuildArtifactPaths": relativeOnbuildArtifactPaths,
		"ImageArtifactPaths":   imageArtifactPaths,
		"PreCopyDirectives":    directives["preCopy"],
		"PostCopyDirectives":   directives["postCopy"],
		"HealthcheckRequired":  healthCheckRequired,
	})

	if err != nil {
		return "", errors.Wrap(err, "Failed to run template")
	}

	dockerfileContents := dockerfileTemplateBuffer.String()

	return dockerfileContents, nil
}

func (b *Builder) gatherArtifactsForSingleStageDockerfile(artifactsDir string,
	processorDockerfileInfo *runtime.ProcessorDockerfileInfo) error {
	buildArgs, err := b.getBuildArgs()
	if err != nil {
		return errors.Wrap(err, "Failed to get build args")
	}

	// create an artifacts directory to which we'll copy all of our stuff
	if err = os.MkdirAll(artifactsDir, 0755); err != nil {
		return errors.Wrap(err, "Failed to create artifacts directory")
	}

	// only pull uhttpc if the platform requires an internal healthcheck client
	if b.platform.GetHealthCheckMode() == platform.HealthCheckModeInternalClient {
		if err = b.ensureImagesExist([]string{uhttpcImage}); err != nil {
			return errors.Wrap(err, "Failed to ensure uhttpc image exists")
		}

		// to support single stage, we need to extract uhttpc and onbuild artifacts ourselves. this means
		// running a container from a certain image and then extracting artifacts
		err = b.dockerClient.CopyObjectsFromImage(uhttpcImage, map[string]string{
			"/home/nuclio/bin/uhttpc": path.Join(b.stagingDir, "artifacts", "uhttpc"),
		}, false)

		if err != nil {
			return errors.Wrap(err, "Failed to copy objects from uhttpc")
		}
	}

	// to facilitate good ux, pull images that we're going to need (and log it) before copying
	// objects from them. this also prevents docker spewing out errors about an image not existing
	if err = b.ensureImagesExist([]string{processorDockerfileInfo.OnbuildImage}); err != nil {
		return errors.Wrap(err, "Failed to ensure required images exist")
	}

	// maps between a path in the onbuild image to a local path in artifacts
	onbuildArtifactPaths := map[string]string{}
	for onbuildArtifactPath := range processorDockerfileInfo.OnbuildArtifactPaths {
		onbuildArtifactPaths[onbuildArtifactPath] = path.Join(artifactsDir, path.Base(onbuildArtifactPath))
	}

	// build an image to trigger the onbuild stuff. then extract the artifacts
	err = b.buildFromAndCopyObjectsFromContainer(processorDockerfileInfo.OnbuildImage,
		onbuildArtifactPaths,
		buildArgs)

	if err != nil {
		return errors.Wrap(err, "Failed to copy objects from onbuild")
	}

	return nil
}

func (b *Builder) ensureImagesExist(images []string) error {
	if b.GetNoBaseImagePull() {
		b.logger.Debug("Skipping base images pull")
		return nil
	}

	for _, image := range images {
		if err := b.dockerClient.PullImage(image); err != nil {
			return errors.Wrap(err, "Failed to pull image")
		}
	}

	return nil
}

func (b *Builder) getBuildArgs() (map[string]string, error) {
	versionInfo, err := version.Get()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get version info")
	}

	buildArgs := map[string]string{}

	if b.options.FunctionConfig.Spec.Build.Offline {
		buildArgs["NUCLIO_BUILD_OFFLINE"] = "true"
	}

	// set tag / arch
	buildArgs["NUCLIO_LABEL"] = versionInfo.Label
	buildArgs["NUCLIO_ARCH"] = versionInfo.Arch

	// set handler dir
	buildArgs["NUCLIO_BUILD_LOCAL_HANDLER_DIR"] = "handler"

	return buildArgs, nil
}

func (b *Builder) buildFromAndCopyObjectsFromContainer(onbuildImage string,
	artifactPaths map[string]string,
	buildArgs map[string]string) error {

	dockerfilePath := path.Join(b.stagingDir, "Dockerfile.onbuild")

	onbuildDockerfileContents := fmt.Sprintf(`FROM %s
ARG NUCLIO_LABEL
ARG NUCLIO_ARCH
`, onbuildImage)

	// generate a simple Dockerfile from the onbuild image
	err := ioutil.WriteFile(dockerfilePath, []byte(onbuildDockerfileContents), 0644)
	if err != nil {
		return errors.Wrapf(err, "Failed to write onbuild Dockerfile to %s", dockerfilePath)
	}

	// log
	b.logger.DebugWith("Generated onbuild Dockerfile", "contents", onbuildDockerfileContents)

	// generate an image name
	onbuildImageName := fmt.Sprintf("nuclio-onbuild-%s", xid.New().String())

	// trigger a build
	err = b.dockerClient.Build(&dockerclient.BuildOptions{
		Image:          onbuildImageName,
		ContextDir:     b.stagingDir,
		BuildArgs:      buildArgs,
		DockerfilePath: dockerfilePath,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to build onbuild image")
	}

	defer b.dockerClient.RemoveImage(onbuildImageName) // nolint: errcheck

	// now that we have an image, we can copy the artifacts from it
	return b.dockerClient.CopyObjectsFromImage(onbuildImageName, artifactPaths, false)
}

func (b *Builder) commandsToDirectives(commands []string) (map[string][]functionconfig.Directive, error) {

	// create directives
	directives := map[string][]functionconfig.Directive{
		"preCopy":  {},
		"postCopy": {},
	}

	// current directive kind starts with "preCopy". If the user specifies @nuclio.postCopy it switches to that
	currentDirective := "preCopy"

	// iterate over commands
	for i := 0; i < len(commands); i++ {
		command := commands[i]

		// check if the last character is backslash. if so treat this and the next command as one multi-line command
		// example: commands = []{"echo 1\", "2\", "3"} -> "RUN echo 123"
		aggregatedCommand := ""
		for {

			if strings.TrimSpace(command) == "@nuclio.postCopy" {
				currentDirective = "postCopy"
				break

			} else if len(command) != 0 && command[len(command)-1] == '\\' {
				// gets here when the current command ends with backslash - concatenate the next command to it

				// remove backslash
				command = command[:len(command)-1]

				aggregatedCommand += command

				// exit the loop if we've processed all the commands
				if len(commands) <= i+1 {
					break
				}

				// check if the next command is continuing the multi-line
				i++
				command = commands[i]

			} else {
				// gets here when the current command is not continuing a multi-line

				aggregatedCommand += command
				break
			}
		}

		// may be true when @nuclio.postCopy is given
		if aggregatedCommand == "" {
			continue
		}

		// add to proper directive. support only RUN
		directives[currentDirective] = append(directives[currentDirective], functionconfig.Directive{
			Kind:  "RUN",
			Value: aggregatedCommand,
		})
	}

	return directives, nil
}

func (b *Builder) mergeDirectives(first map[string][]functionconfig.Directive,
	second map[string][]functionconfig.Directive) map[string][]functionconfig.Directive {

	keys := []string{"preCopy", "postCopy"}
	merged := map[string][]functionconfig.Directive{}

	for _, key := range keys {

		// iterate over inputs
		for _, input := range []map[string][]functionconfig.Directive{
			first,
			second,
		} {

			// add all directives from input into merged
			merged[key] = append(merged[key], input[key]...)
		}
	}

	return merged
}

func (b *Builder) getSourceCodeFromFilePath() (string, error) {

	// if the file path is after resolving function source code, do nothing
	if b.options.FunctionConfig.Spec.Build.FunctionSourceCode != "" {
		return "", errors.New("Function source code already exists")
	}

	// if the file path extension is of certain binary types, ignore
	if path.Ext(b.options.FunctionConfig.Spec.Build.Path) == ".jar" {
		return "", errors.New("Function source code cannot be extracted from this file type")
	}

	// if user supplied a file containing printable only characters (i.e. not a zip, jar, etc) - copy the contents
	// to functionSourceCode so that the dashboard may display it
	functionContents, err := ioutil.ReadFile(b.options.FunctionConfig.Spec.Build.Path)
	if err != nil {
		return "", errors.Wrap(err, "Failed to read file contents to source code")
	}

	return string(functionContents), nil
}

// replaces the registry url if applicable. e.g. quay.io/nuclio/some-image:0.0.1 -> 10.0.0.1:2000/some-image:0.0.1
func (b *Builder) renderDependantImageURL(imageURL string, dependantImagesRegistryURL string) (string, error) {
	if dependantImagesRegistryURL == "" {
		return imageURL, nil
	}

	// be tolerant of trailing slash in dependantImagesRegistryURL
	dependantImagesRegistryURL = strings.TrimSuffix(dependantImagesRegistryURL, "/")

	// take the image part of the url
	splitImageURL := strings.Split(imageURL, "/")
	imageNameAndTag := splitImageURL[len(splitImageURL)-1]

	renderedImageURL := dependantImagesRegistryURL + "/" + imageNameAndTag

	b.logger.DebugWith("Rendering dependant image URL",
		"imageURL", imageURL,
		"dependantImagesRegistryURL", dependantImagesRegistryURL,
		"renderedImageURL", renderedImageURL)

	return renderedImageURL, nil
}

func (b *Builder) getFileExtensionByURL(inputURL string) (string, error) {

	// parse the url
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", errors.Wrap(err, "Failed to parse URL")
	}

	return path.Ext(parsedURL.Path), nil
}

func (b *Builder) resolveFunctionPathFromURL(functionPath string, codeEntryType string) (string, error) {
	var err error

	if common.IsURL(functionPath) || codeEntryType == S3EntryType {
		if codeEntryType == GithubEntryType {
			functionPath, err = b.getFunctionPathFromGithubURL(functionPath)
			if err != nil {
				return "", errors.Wrapf(err, "Failed to infer function path of github entry type")
			}
		}

		isArchive := codeEntryType == S3EntryType ||
			codeEntryType == ArchiveEntryType ||
			codeEntryType == GithubEntryType

		tempDir, err := b.mkDirUnderTemp("download")
		if err != nil {
			return "", errors.Wrapf(err, "Failed to create temporary dir for download: %s", tempDir)
		}

		tempFile, err := b.getFunctionTempFile(tempDir, functionPath, isArchive)
		if err != nil {
			return "", errors.Wrap(err, "Failed to get function temporary file")
		}

		b.logger.DebugWith("Created local temporary function file for URL",
			"path", functionPath,
			"codeEntryType", codeEntryType,
			"tempFileName", tempFile.Name())

		switch codeEntryType {
		case S3EntryType:
			err = b.downloadFunctionFromS3(tempFile)
		default:
			err = b.downloadFunctionFromURL(tempFile, functionPath, codeEntryType)
		}

		if err != nil {
			return "", errors.Wrap(err, "Failed to download file")
		}

		if (codeEntryType == S3EntryType || codeEntryType == GithubEntryType || codeEntryType == ArchiveEntryType) &&
			!util.IsCompressed(tempFile.Name()) {
			return "", errors.New("Downloaded file type is not supported. (expected an archive)")
		}

		return tempFile.Name(), nil
	}

	return functionPath, nil
}

func (b *Builder) downloadFunctionFromS3(tempFile *os.File) error {
	s3Attributes, err := b.validateAndParseS3Attributes(b.options.FunctionConfig.Spec.Build.CodeEntryAttributes)
	if err != nil {
		return errors.Wrap(err, "Failed to parse and validate s3 code entry attributes")
	}

	err = b.s3Client.Download(tempFile,
		s3Attributes["s3Bucket"],
		s3Attributes["s3ItemKey"],
		s3Attributes["s3Region"],
		s3Attributes["s3AccessKeyId"],
		s3Attributes["s3SecretAccessKey"],
		s3Attributes["s3SessionToken"])

	if err != nil {
		return errors.Wrap(err, "Failed to download the function archive from s3")
	}

	return nil
}

func (b *Builder) downloadFunctionFromURL(tempFile *os.File,
	functionPath string,
	codeEntryType string) error {
	userDefinedHeaders, found := b.options.FunctionConfig.Spec.Build.CodeEntryAttributes["headers"]
	headers := http.Header{}

	if found {

		// guaranteed a map with key of type string, the values need to be checked for correctness
		for key, value := range userDefinedHeaders.(map[string]interface{}) {
			stringValue, ok := value.(string)
			if !ok {
				return errors.New("Failed to convert header value to string")
			}
			headers.Set(key, stringValue)
		}
	}

	b.logger.DebugWith("Downloading function",
		"url", functionPath,
		"target", tempFile.Name(),
		"headers", headers)

	return common.DownloadFile(functionPath, tempFile, headers)
}

func (b *Builder) getFunctionTempFile(tempDir string, functionPath string, isArchive bool) (*os.File, error) {
	functionPathBase := path.Base(functionPath)

	// for archives, use a temporary local file renamed to something short to allow wacky long archive URLs
	if isArchive || util.IsCompressed(functionPathBase) {

		// retain file extension
		fileExtension, err := b.getFileExtensionByURL(functionPath)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get file extension from URL")
		}

		return ioutil.TempFile(tempDir, "nuclio-function-*"+fileExtension)
	}

	// for non-archives, must retain file name
	return os.OpenFile(path.Join(tempDir, functionPathBase), os.O_RDWR|os.O_CREATE, 0600)
}
