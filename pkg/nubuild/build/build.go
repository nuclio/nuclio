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
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/nubuild/eventhandlerparser"
	"github.com/nuclio/nuclio/pkg/nubuild/util"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type Options struct {
	Verbose         bool
	FunctionPath    string
	OutputType      string
	OutputName      string
	Version         string
	NuclioSourceDir string
	NuclioSourceURL string
	PushRegistry    string
}

// returns the directory the function is in
func (o *Options) getFunctionDir() string {

	// if the function directory was passed, just return that. if the function path was passed, return the directory
	// the function is in
	if isDir(o.FunctionPath) {
		return o.FunctionPath
	} else {
		return path.Dir(o.FunctionPath)
	}
}

type Builder struct {
	logger  nuclio.Logger
	options *Options
}

const (
	defaultBuilderImage     = "golang:1.8"
	defaultProcessorImage   = "alpine"
	processorConfigFileName = "processor.yaml"
	buildConfigFileName     = "build.yaml"
	nuclioDockerDir         = "/opt/nuclio"
)

type config struct {
	Name    string `mapstructure:"name"`
	Handler string `mapstructure:"handler"`
	Build   struct {
		Image     string   `mapstructure:"image"`
		Script    string   `mapstructure:"script"`
		Commands  []string `mapstructure:"commands"`
		Copy      []string `mapstructure:"copy"`
		NuclioDir string
	} `mapstructure:"build"`
}

type buildStep struct {
	Message string
	Func    func() error
}

func findFunctionPath(options *Options, path string) (err error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		out, err := ioutil.TempFile("", "")
		if err != nil {
			return
		}
		defer out.Close()

		response, err := http.Get(path)
		if err != nil {
			return
		}
		defer response.Body.Close()

		_, err = io.Copy(out, response.Body)
		if err != nil {
			return
		}
		options.FunctionPath = out.Name()
	} else {
		// Assume it's a local path
		options.FunctionPath, err = filepath.Abs(filepath.Clean(path))
	}
	return
}

func NewBuilder(parentLogger nuclio.Logger, options *Options, path string) (*Builder, error) {
	if err := findFunctionPath(options, path); err != nil {
		return nil, err
	}
	return &Builder{
		logger:  parentLogger.GetChild("builder").(nuclio.Logger),
		options: options,
	}, nil
}

func (b *Builder) Build() error {

	// create a configuration given the path to the function. the path can either be a directory holding one
	// or more files (at the very least a Go file holding handlers, an optional processor.yaml and an optional
	// build.yaml) or the actual function source
	config, err := b.createConfig(b.options.FunctionPath)

	if err != nil {
		return errors.Wrap(err, "Unable to create configuration")
	}

	b.logger.Info("Building run environment")

	env, err := newEnv(b.logger, config, b.options)
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

	b.logger.DebugWith("Outputting",
		"output_type", b.options.OutputType,
		"output_name", env.outputName)

	return nil
}

func (b *Builder) buildDockerSteps(env *env, outputToImage bool) error {
	b.logger.Debug("Creating docker helper")

	docker, err := newDockerHelper(b.logger, env)
	if err != nil {
		return errors.Wrap(err, "Error building docker helper")
	}

	defer docker.close()

	buildSteps := []buildStep{
		{Message: "Running docker onbuild",
			Func: docker.createOnBuildImage},
		{Message: "Running docker binary build",
			Func: docker.createBuilderImage},
	}

	if outputToImage {
		buildSteps = append(buildSteps, buildStep{
			Message: "Creating output container " + env.outputName,
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

func (b *Builder) readKeyFromConfigFile(c *config, key string, fileName string) error {
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

	if err := v.Unmarshal(c); err != nil {
		return errors.Wrapf(err, "Unable to unmarshal %q configuration", fileName)
	}
	return nil
}

func (b *Builder) readProcessorConfigFile(c *config, fileName string) error {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		return nil
	}

	// try to read the configuration file
	return b.readKeyFromConfigFile(c, "function", fileName)
}

func (b *Builder) readBuildConfigFile(c *config, fileName string) error {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		return nil
	}

	// try to read the configuration file
	return b.readKeyFromConfigFile(c, "", fileName)
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

func (b *Builder) populateEventHandlerInfo(functionPath string, cfg *config) error {
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

	return nil
}

func (b *Builder) createConfig(functionPath string) (*config, error) {

	// initialize config and populate with defaults.
	config := config{}
	config.Build.Image = defaultProcessorImage
	config.Build.Commands = []string{}
	config.Build.Script = ""

	// if the function path is a directory - try to look for processor.yaml / build.yaml lurking around there
	// if it's not a directory, we'll assume we got the path to the actual source
	if isDir(functionPath) {

		// seeing how the path is a dir, lets look for some
		processorConfigPath := filepath.Join(functionPath, processorConfigFileName)
		buildConfigPath := filepath.Join(functionPath, buildConfigFileName)

		if err := b.readProcessorConfigFile(&config, processorConfigPath); err != nil {
			return nil, err
		}

		if err := b.readBuildConfigFile(&config, buildConfigPath); err != nil {
			return nil, err
		}
	}

	// if we did not find any handers or name the function - try to parse source golang code looking for
	// functions
	if len(config.Handler) == 0 || len(config.Name) == 0 {
		if err := b.populateEventHandlerInfo(b.options.FunctionPath, &config); err != nil {
			return nil, err
		}
	}

	config.Build.NuclioDir = nuclioDockerDir
	return &config, nil
}
