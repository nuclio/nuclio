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
	"os"
	"path"
	"path/filepath"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/build/eventhandlerparser"
	"github.com/nuclio/nuclio/pkg/processor/build/util"
	"github.com/nuclio/nuclio/pkg/processor/config"
	"github.com/nuclio/nuclio/pkg/util/common"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type Options struct {
	FunctionPath    string
	NuclioSourceDir string
	NuclioSourceURL string
	OutputName      string
	OutputType      string
	PushRegistry    string
	Runtime         string
	Verbose         bool
	Version         string
}

// returns the directory the function is in
func (o *Options) getFunctionDir() string {

	// if the function directory was passed, just return that. if the function path was passed, return the directory
	// the function is in
	if common.IsDir(o.FunctionPath) {
		return o.FunctionPath
	}

	return path.Dir(o.FunctionPath)
}

type Builder struct {
	logger  nuclio.Logger
	options *Options
}

const (
	processorConfigFileName = "processor.yaml"
	buildConfigFileName     = "build.yaml"
	nuclioDockerDir         = "/opt/nuclio"
)

type configuration struct {
	Name    string
	Handler string
	Runtime string
	FunctionPath string
	Build   struct {
		ProcessorBaseImage     string
		Script    string
		Commands  []string
		Copy      map[string]string
		NuclioDir string
	}
}

type buildStep struct {
	Message string
	Func    func() error
}

func NewBuilder(parentLogger nuclio.Logger, options *Options) *Builder {
	return &Builder{
		logger:  parentLogger,
		options: options,
	}
}

func (b *Builder) Build() error {
	var err error

	// if the function path is a URL, resolve it to a local file
	b.options.FunctionPath, err = b.resolveFunctionPath(b.options.FunctionPath)
	if err != nil {
		return errors.Wrap(err, "Failed to resolve funciton path")
	}

	// try to understand which runtime we're going to build
	b.options.Runtime, err = b.resolveRuntime(b.options)
	if err != nil {
		return errors.Wrap(err, "Failed to resolve runtime")
	}

	// create a configuration given the path to the function. the path can either be a directory holding one
	// or more files (at the very least a Go file holding handlers, an optional processor.yaml and an optional
	// build.yaml) or the actual function source
	cfg, err := b.createConfig(b.options.Runtime, b.options.FunctionPath)

	if err != nil {
		return errors.Wrap(err, "Unable to create configuration")
	}

	b.logger.Info("Preparing environment")

	env, err := newEnv(b.logger, cfg, b.options)
	if err != nil {
		return errors.Wrap(err, "Failed to create env")
	}

	if err := b.buildDockerSteps(env, b.options.OutputType == "docker"); err != nil {
		return err
	}

	if b.options.OutputType == "binary" {
		if err := util.CopyFile(env.getBinaryPath(), env.outputName); err != nil {
			return err
		}
	}

	b.logger.InfoWith("Build completed successfully",
		"type", b.options.OutputType,
		"name", env.outputName)

	return nil
}

func (b *Builder) buildDockerSteps(env *env, outputToImage bool) error {
	b.logger.Debug("Creating docker helper")

	docker, err := newDockerHelper(b.logger, env)
	if err != nil {
		return errors.Wrap(err, "Error building docker helper")
	}

	buildSteps := []buildStep{
		{Message: "Preparing docker base images", Func: docker.createOnBuildImage},
		{Message: "Building processor (in docker)", Func: docker.createProcessorBinary},
	}

	if outputToImage {
		buildSteps = append(buildSteps, buildStep{
			Message: fmt.Sprintf("Dockerizing processor binary (%s)", env.outputName),
			Func:    docker.createProcessorImage})
	}

	for _, step := range buildSteps {
		b.logger.Info(step.Message)
		if err := step.Func(); err != nil {
			return errors.Wrap(err, "Error while "+step.Message)
		}
	}

	return nil
}

func (b *Builder) readKeyFromConfigFile(cfg *configuration, key string, fileName string) error {
	b.logger.DebugWith("Reading config file", "path", fileName, "key", key)

	v := viper.New()
	v.SetConfigFile(fileName)
	if err := v.ReadInConfig(); err != nil {
		return errors.Wrapf(err, "Unable to read %q configuration", fileName)
	}

	if key != "" {
		v = v.Sub(key)

		if v == nil {
			return fmt.Errorf("Configuration file %s has no key %s", fileName, key)
		}
	}

	if err := v.Unmarshal(cfg); err != nil {
		return errors.Wrapf(err, "Unable to unmarshal %q configuration", fileName)
	}
	return nil
}

func (b *Builder) readProcessorConfigFile(cfg *configuration, fileName string) error {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		return nil
	}

	processorConfig, err := config.ReadProcessorConfiguration(fileName)
	if err != nil {
		return err
	}

	functionConfig := processorConfig["function"]
	mapping := map[string]*string{
		"handler": &cfg.Handler,
		"name":    &cfg.Name,
		"kind":    &cfg.Runtime,
	}

	for key, valp := range mapping {
		val := functionConfig.GetString(key)
		if len(val) == 0 {
			continue
		}
		*valp = val
	}

	return nil
}

func (b *Builder) readBuildConfigFile(cfg *configuration, fileName string) error {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		return nil
	}

	// try to read the configuration file
	return b.readKeyFromConfigFile(cfg, "", fileName)
}

func adjective(n int) string {
	switch n {
	case 0:
		return "no"
	case 1: // noop
	default:
		return "too many"
	}
	return "" // make compiler happy
}

func (b *Builder) isMissingHandlerInfo(cfg *configuration) bool {
	return len(cfg.Handler) == 0 || len(cfg.Name) == 0
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

func (b *Builder) createConfig(runtime string, functionPath string) (*configuration, error) {

	// initialize config and populate with defaults.
	cfg := &configuration{}
	cfg.Build.Commands = []string{}
	cfg.Build.Script = ""
	cfg.Runtime = runtime
	cfg.FunctionPath = functionPath

	// get the image based on the runtime
	cfg.Build.ProcessorBaseImage = b.getDefaultRuntimeProcessorBaseImage(cfg.Runtime)

	// if the function path is a directory - try to look for processor.yaml / build.yaml lurking around there
	// if it's not a directory, we'll assume we got the path to the actual source
	if common.IsDir(cfg.FunctionPath) {

		// seeing how the path is a dir, lets look for some
		processorConfigPath := filepath.Join(cfg.FunctionPath, processorConfigFileName)
		buildConfigPath := filepath.Join(cfg.FunctionPath, buildConfigFileName)

		if err := b.readProcessorConfigFile(cfg, processorConfigPath); err != nil {
			return nil, err
		}

		if err := b.readBuildConfigFile(cfg, buildConfigPath); err != nil {
			return nil, err
		}
	}

	// do we know the name of the processor and have a handler name?
	if b.isMissingHandlerInfo(cfg) {
		b.populateEventHandlerInfo(cfg)
	}

	cfg.Build.NuclioDir = nuclioDockerDir

	return cfg, nil
}

func (b *Builder) resolveRuntime(options *Options) (string, error) {

	// if runtime is set, just use that
	if options.Runtime != "" {
		return options.Runtime, nil
	}

	// if the function path is a directory, assume Go for now
	if common.IsDir(options.FunctionPath) {
		return "go", nil
	}

	// try to read the file extension
	functionFileExtension := filepath.Ext(options.FunctionPath)

	// if the file extension is of a known runtime, use that (skip dot in extension)
	switch functionFileExtension[1:] {
	case "go":
		return "go", nil
	case "py":
		return "python", nil
	default:
		return "", fmt.Errorf("No supported runtime for file extension %s", functionFileExtension)
	}
}

func (b *Builder) getDefaultRuntimeProcessorBaseImage(runtime string) string {
	switch runtime {
	case "python":
		return "python:3-alpine"
	default:
		return "alpine"
	}
}

func (b *Builder) populateEventHandlerInfo(cfg *configuration) error {
	switch cfg.Runtime {
	case "go":
		if err := b.populateGoEventHandlerInfo(cfg.FunctionPath, cfg); err != nil {
			return err
		}
	case "python":
		if cfg.Handler == "" {
			cfg.Handler = "handler"
		}

		if cfg.Name == "" {
			cfg.Name = b.options.OutputName
		}

	default:
		return errors.New("Unsupported runtime")
	}

	return nil
}

func (b *Builder) populateGoEventHandlerInfo(functionPath string, cfg *configuration) error {
	parser := eventhandlerparser.NewEventHandlerParser(b.logger)
	packages, handlers, err := parser.ParseEventHandlers(functionPath)
	if err != nil {
		errors.Wrapf(err, "Can't find handlers in %q", functionPath)
	}

	b.logger.DebugWith("Parsed event handlers", "packages", packages, "handlers", handlers)

	if len(handlers) != 1 {
		adj := adjective(len(handlers))
		return errors.Wrapf(err, "%s handlers found in %q", adj, functionPath)
	}

	if len(packages) != 1 {
		adj := adjective(len(packages))
		return errors.Wrapf(err, "%s packages found in %q", adj, functionPath)
	}

	if len(cfg.Handler) == 0 {
		cfg.Handler = handlers[0]
		b.logger.DebugWith("Selected handler", "handler", cfg.Handler)
	}

	if len(cfg.Name) == 0 {
		cfg.Name = packages[0]
		b.logger.DebugWith("Selected package", "package", cfg.Name)
	}

	if b.isMissingHandlerInfo(cfg) {
		return fmt.Errorf("No handler information found")
	}

	return nil
}

