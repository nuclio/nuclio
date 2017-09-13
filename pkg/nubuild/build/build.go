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
	"text/template"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/nubuild/eventhandlerparser"
	"github.com/nuclio/nuclio/pkg/nubuild/util"
	processorconfig "github.com/nuclio/nuclio/pkg/processor/config"
	"github.com/nuclio/nuclio/pkg/util/cmdrunner"
	"github.com/nuclio/nuclio/pkg/util/common"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// Options are build options
type Options struct {
	FunctionPath     string
	OutputName       string
	OutputType       string
	ProcessorURL     string
	PythonWrapperURL string
	PushRegistry     string
	Verbose          bool
	Version          string
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
	defaultHandlerDLL       = "handler.so"
)

type buildConfig struct {
	Name        string `mapstructure:"name"`
	Handler     string `mapstructure:"handler"`
	Runtime     string `mapstructure:"kind"`
	ConfigFiles []string
	Build       struct {
		Make     string   `mapstructure:"make"`
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
		b.getNuclioFiles,
		b.ensureProcessorConfig,
		b.buildArtifact,
		b.createDockerfile,
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
	tmpDirPath, err := ioutil.TempDir("", "nuctl-build-")
	if err != nil {
		return err
	}

	// We place everything under "src" to set can set GOPATH to tmpDir
	workDirPath := filepath.Join(tmpDirPath, "src")
	err = os.Mkdir(workDirPath, 0777)
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
	config.Name = defaultHandlerDLL
	config.ConfigFiles = []string{processorConfigFileName}

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

func (b *Builder) getNuclioFiles() error {
	b.logger.DebugWith("Getting processor", "URL", b.options.ProcessorURL)
	u, err := url.Parse(b.options.ProcessorURL)
	if err != nil {
		return err
	}

	processorPath := filepath.Join(b.workDirPath, "processor")

	switch u.Scheme {
	case "file", "":
		err = util.CopyFile(b.options.ProcessorURL, processorPath)
	case "http", "https":
		err = b.downloadProcessor(processorPath)
	default:
		err = fmt.Errorf("Unknown scheme in %q", b.options.ProcessorURL)
	}

	if err != nil {
		return err
	}

	b.logger.DebugWith("Getting python wrapper", "URL", b.options.PythonWrapperURL)
	wrapperBase := "wrapper.py"
	pythonWrapperPath := filepath.Join(b.workDirPath, wrapperBase)
	switch u.Scheme {
	case "file", "":
		err = util.CopyFile(b.options.PythonWrapperURL, pythonWrapperPath)
	case "http", "https":
		err = common.DownloadFile(b.options.PythonWrapperURL, pythonWrapperPath)
	default:
		return fmt.Errorf("Unknown scheme in %q", b.options.ProcessorURL)
	}

	if err != nil {
		return err
	}

	b.config.Build.Copy = append(b.config.Build.Copy, wrapperBase)
	return nil
}

func (b *Builder) ensureProcessorConfig() error {
	processorConfigPath := filepath.Join(b.workDirPath, processorConfigFileName)
	if util.IsFile(processorConfigPath) {
		return nil
	}

	processorConfig := map[string]interface{}{
		"handler": b.config.Handler,
		"path":    filepath.Join(nuclioDockerDir, defaultHandlerDLL),
		"kind":    b.config.Runtime,
	}

	outFile, err := os.Create(processorConfigPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = outFile.Write([]byte("# Generated by nuclio\n\n"))
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(processorConfig)
	if err != nil {
		return err
	}

	_, err = outFile.Write(data)
	if err != nil {
		return err
	}

	return outFile.Close()
}

func (b *Builder) buildArtifact() error {
	if b.config.Runtime != goRuntimeName {
		return nil
	}

	// b.workDir is /tmp/<blah>/src
	goPath := filepath.Dir(b.workDirPath)

	makeCommand := b.config.Build.Make
	if len(makeCommand) == 0 {
		if err := b.buildGoHandler(goPath); err != nil {
			return err
		}
	} else {
		b.logger.DebugWith("Building artifact", "command", makeCommand)
		cmd, err := cmdrunner.NewCmdRunner(b.logger)
		if err != nil {
			return err
		}

		runOptions := &cmdrunner.RunOptions{
			WorkingDir: &b.workDirPath,
			Env: map[string]string{
				"PATH":   os.Getenv("PATH"),
				"GOPATH": goPath,
			},
		}
		if _, err = cmd.Run(runOptions, makeCommand); err != nil {
			return errors.Wrap(err, "Can't build artifact")
		}
	}

	return nil
}

func (b *Builder) buildGoHandler(goPath string) error {
	if b.config.Runtime != goRuntimeName {
		return nil
	}

	b.logger.DebugWith("Building Go handler")
	cmd, err := cmdrunner.NewCmdRunner(b.logger)
	if err != nil {
		return err
	}

	runOptions := &cmdrunner.RunOptions{
		WorkingDir: &b.workDirPath,
		Env: map[string]string{
			"GOOS":        "linux",
			"GOARCH":      "amd64",
			"CGO_ENABLED": "0",
			"GOPATH":      goPath,
			"PATH":        os.Getenv("PATH"),
		},
	}

	_, err = cmd.Run(runOptions, "go get github.com/nuclio/nuclio-sdk")
	if err != nil {
		return err
	}
	_, err = cmd.Run(runOptions, "go build -ldflags='-s -w' -buildmode=plugin -o %s .", defaultHandlerDLL)
	if err != nil {
		return err
	}

	b.config.Build.Copy = append(b.config.Build.Copy, defaultHandlerDLL)
	return nil
}

func (b *Builder) createDockerfile() error {
	funcMap := template.FuncMap{
		"IsDir":    util.IsDir,
		"Basename": filepath.Base,
	}

	dockerTemplate, err := template.New("").Funcs(funcMap).Parse(dockerfileTemplateText)
	if err != nil {
		return errors.Wrap(err, "Can't parse Dockerfile template")
	}

	dockerfilePath := filepath.Join(b.workDirPath, "Dockerfile")
	file, err := os.Create(dockerfilePath)
	if err != nil {
		return errors.Wrapf(err, "Can't create Dockerfile at %q", dockerfilePath)
	}

	defer file.Close()

	params := map[string]interface{}{
		"Image":       b.config.Build.Image,
		"Copy":        b.config.Build.Copy,
		"Script":      b.config.Build.Script,
		"Commands":    b.config.Build.Commands,
		"ConfigFiles": b.config.ConfigFiles,

		// TODO: From env/config
		"OptDir": nuclioDockerDir,
		"EtcDir": "/etc/nuclio",
	}

	if err := dockerTemplate.Execute(file, params); err != nil {
		return errors.Wrapf(err, "Can't execurte template with %#v", params)
	}

	return file.Close()
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

	b.populateConfigFiles(processorConfig)

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

func (b *Builder) populateConfigFiles(processorConfig map[string]*viper.Viper) {
	for _, section := range processorConfig {
		for _, key := range section.AllKeys() {
			if key == "config_path" {
				fileName := section.GetString(key)
				b.config.ConfigFiles = append(b.config.ConfigFiles, fileName)
			}
		}
	}
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
