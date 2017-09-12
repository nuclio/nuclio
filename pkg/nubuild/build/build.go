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
	"compress/bzip2"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/nubuild/eventhandlerparser"
	"github.com/nuclio/nuclio/pkg/nubuild/util"
	processorconfig "github.com/nuclio/nuclio/pkg/processor/config"
	"github.com/nuclio/nuclio/pkg/util/common"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// Options are build options
type Options struct {
	FunctionPath string
	OutputName   string
	OutputType   string
	ProcessorURL string
	PushRegistry string
	Verbose      bool
	Version      string
}

// Builder builds processor docker images
type Builder struct {
	logger      nuclio.Logger
	options     *Options
	config      *buildConfig
	workDirPath string
}

const (
	defaultBuilderImage     = "golang:1.8"
	defaultProcessorImage   = "alpine"
	processorConfigFileName = "processor.yaml"
	buildConfigFileName     = "build.yaml"
	nuclioDockerDir         = "/opt/nuclio"
	goRuntimeName           = "go"
)

type buildConfig struct {
	Name    string `mapstructure:"name"`
	Handler string `mapstructure:"handler"`
	Runtime string `mapstructure:"kind"`
	Build   struct {
		Image    string   `mapstructure:"image"`
		Script   string   `mapstructure:"script"`
		Commands []string `mapstructure:"commands"`
		Copy     []string `mapstructure:"copy"`
	} `mapstructure:"build"`
}

// NewBuilder returns a new nuclio processor builder
func NewBuilder(parentLogger nuclio.Logger, options *Options) *Builder {
	return &Builder{
		logger:  parentLogger,
		options: options,
	}
}

// FunctionName return the name of "fn"
func FunctionName(fn interface{}) string {
	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()

	// Unmangle name: main.(*Builder).(main.createWorkDir)-fm -> createWorkDir
	methodSuffix := ")-fm"
	if strings.HasSuffix(name, methodSuffix) {
		name = name[:len(name)-len(methodSuffix)]
	}

	i := strings.LastIndex(name, ".")
	if i > -1 {
		name = name[i+1:]
	}

	return name
}

// Build builds the processor docker image
func (b *Builder) Build() error {
	var err error

	// if the function path is a URL, resolve it to a local file
	b.options.FunctionPath, err = b.resolveFunctionPath(b.options.FunctionPath)
	if err != nil {
		return err
	}

	steps := []func() error{
		b.createWorkDir,
		b.readConfiguration,
		b.getProcessor,
		b.ensureProcessorConfig,
	}

	for _, step := range steps {
		b.logger.DebugWith("Running build step", "name", FunctionName(step))
		if err := step(); err != nil {
			return err
		}
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
	}

	// Assume it's a local path
	return filepath.Abs(filepath.Clean(functionPath))
}

func (b *Builder) createWorkDir() error {
	workDirPath, err := ioutil.TempDir("", "nuctl-build-")
	if err != nil {
		return err
	}

	b.workDirPath = workDirPath
	b.logger.DebugWith("Created working directory", "path", workDirPath)
	functionPath := b.options.FunctionPath
	if util.IsFile(functionPath) {
		destPath := filepath.Join(workDirPath, filepath.Base(functionPath))
		return util.CopyFile(functionPath, destPath)
	}

	return util.CopyDirContent(functionPath, workDirPath)
}

func (b *Builder) readConfiguration() error {
	// initialize config and populate with defaults.
	config := &buildConfig{}
	config.Build.Image = defaultProcessorImage
	config.Runtime = goRuntimeName
	config.Name = "handler.so"

	steps := []func(*buildConfig) error{
		b.readBuildConfig,
		b.readProcessorConfig,
		b.populateEventHandlerInfo,
	}

	for _, step := range steps {
		if err := step(config); err != nil {
			return err
		}
	}

	b.config = config

	return nil
}

func (b *Builder) getProcessor() error {
	u, err := url.Parse(b.options.ProcessorURL)
	if err != nil {
		return err
	}

	processorPath := filepath.Join(b.workDirPath, "processor")

	switch u.Scheme {
	case "file", "":
		return util.CopyFile(b.options.ProcessorURL, processorPath)
	case "http", "https":
		return b.downloadProcessor(processorPath)
	default:
		return fmt.Errorf("Unknown scheme in %q", b.options.ProcessorURL)
	}
}

func (b *Builder) ensureProcessorConfig() error {
	processorConfigPath := filepath.Join(b.workDirPath, processorConfigFileName)
	if util.IsFile(processorConfigPath) {
		return nil
	}

	processorConfig := map[string]interface{}{
		"handler": b.config.Handler,
		"kind":    b.config.Runtime,
	}

	data, err := yaml.Marshal(processorConfig)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(processorConfigPath, data, 0666)
}

func (b *Builder) readBuildConfig(config *buildConfig) error {
	buildConfigPath := filepath.Join(b.workDirPath, buildConfigFileName)
	if !util.IsFile(buildConfigPath) {
		return nil
	}

	v := viper.New()
	v.SetConfigFile(buildConfigPath)
	if err := v.ReadInConfig(); err != nil {
		return errors.Wrapf(err, "Unable to read %q configuration", buildConfigPath)
	}

	if err := v.Unmarshal(config); err != nil {
		return errors.Wrapf(err, "Unable to unmarshal %q configuration", buildConfigPath)
	}
	return nil
}

func (b *Builder) readProcessorConfig(config *buildConfig) error {
	processorConfigPath := filepath.Join(b.workDirPath, processorConfigFileName)
	if !util.IsFile(processorConfigPath) {
		return nil
	}
	processorConfig, err := processorconfig.ReadProcessorConfiguration(processorConfigPath)
	if err != nil {
		return errors.Wrapf(err, "Can't read processor configuration from %q", processorConfigPath)
	}

	functionConfig := processorConfig["function"]
	if functionConfig == nil {
		return nil
	}

	mapping := map[string]*string{
		"kind":    &config.Runtime,
		"handler": &config.Handler,
		"name":    &config.Name,
	}

	for key, varp := range mapping {
		if value := functionConfig.GetString(key); value != "" {
			*varp = value
		}
	}

	return nil
}

func (b *Builder) populateEventHandlerInfo(config *buildConfig) error {
	if config.Runtime != goRuntimeName {
		return nil
	}

	if !b.isMissingHandlerInfo(config) {
		return nil
	}

	parser := eventhandlerparser.NewEventHandlerParser(b.logger)
	handlers, err := parser.ParseEventHandlers(b.options.FunctionPath)
	if err != nil {
		errors.Wrapf(err, "Can't find handlers in %q", b.options.FunctionPath)
	}

	b.logger.DebugWith("Parsed event handlers", "handlers", handlers)

	if len(handlers) != 1 {
		adjective := "no"
		if len(handlers) > 1 {
			adjective = "too many"
		}
		return errors.Wrapf(err, "%s handlers found in %q", adjective, b.options.FunctionPath)
	}

	if len(config.Handler) == 0 {
		config.Handler = handlers[0]
		b.logger.DebugWith("Selected handler", "handler", config.Handler)
	}

	if b.isMissingHandlerInfo(config) {
		return fmt.Errorf("No handler information found")
	}

	return nil
}

func (b *Builder) isMissingHandlerInfo(cfg *buildConfig) bool {
	return len(cfg.Handler) == 0 || len(cfg.Name) == 0
}

func (b *Builder) downloadProcessor(processorPath string) error {
	b.logger.DebugWith("Downloading processor", "URL", b.options.ProcessorURL)
	isCompressed := strings.HasSuffix(b.options.ProcessorURL, ".bz2")

	downloadPath := processorPath
	if isCompressed {
		tmpFile, err := ioutil.TempFile("", "nuclio-processor-")
		if err != nil {
			return err
		}
		tmpFile.Close()
		downloadPath = tmpFile.Name()
	}

	if err := common.DownloadFile(b.options.ProcessorURL, downloadPath); err != nil {
		return err
	}

	if isCompressed {
		if err := b.decompress(downloadPath, processorPath); err != nil {
			return err
		}
	}

	return os.Chmod(processorPath, 0555)
}

func (b *Builder) decompress(srcPath, destPath string) error {
	inFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}

	defer inFile.Close()
	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}

	defer outFile.Close()

	reader := bzip2.NewReader(inFile)
	_, err = io.Copy(outFile, reader)
	if err != nil {
		return err
	}

	return outFile.Close()
}

/*

    // create a configuration given the path to the function. the path can either be a directory holding one
    // or more files (at the very least a Go file holding handlers, an optional processor.yaml and an optional
    // build.yaml) or the actual function source
    config, err := b.createConfig(b.options.FunctionPath)

    if err != nil {
        return errors.Wrap(err, "Unable to create configuration")
    }

    b.logger.Info("Preparing environment")

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




func (b *Builder) createConfig(functionPath string) (*config, error) {

    // initialize config and populate with defaults.
    config := &config{}
    config.Build.Image = defaultProcessorImage
    config.Build.Commands = []string{}
    config.Build.Script = ""

    // if the function path is a directory - try to look for processor.yaml / build.yaml lurking around there
    // if it's not a directory, we'll assume we got the path to the actual source
    if isDir(functionPath) {

        // seeing how the path is a dir, lets look for some
        processorConfigPath := filepath.Join(functionPath, processorConfigFileName)
        buildConfigPath := filepath.Join(functionPath, buildConfigFileName)

        if err := b.readProcessorConfigFile(config, processorConfigPath); err != nil {
            return nil, err
        }

        if err := b.readBuildConfigFile(config, buildConfigPath); err != nil {
            return nil, err
        }
    }

    // if we did not find any handers or name the function - try to parse source golang code looking for
    // functions
    if b.isMissingHandlerInfo(config) {
        if err := b.populateEventHandlerInfo(functionPath, config); err != nil {
            return nil, err
        }
    }

    if b.isMissingHandlerInfo(config) {
        return nil, fmt.Errorf("No handler information found")
    }

    config.Build.NuclioDir = nuclioDockerDir

    return config, nil
}
*/
