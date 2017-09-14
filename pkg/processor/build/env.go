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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/build/util"
	processorconfig "github.com/nuclio/nuclio/pkg/processor/config"
	"github.com/nuclio/nuclio/pkg/util/cmdrunner"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

var (
	userFunctionPath         = []string{"cmd", "processor", "user_functions"}
	userFunctionRegistryPath = []string{"cmd", "processor"}
)

type env struct {
	logger           nuclio.Logger
	cmdRunner        *cmdrunner.CmdRunner
	config           *config
	options          *Options
	outputName       string
	workDir          string
	nuclioDestDir    string
	userFunctionPath string
}

func newEnv(parentLogger nuclio.Logger, config *config, options *Options) (*env, error) {
	var err error

	env := &env{
		logger:  parentLogger.GetChild("env").(nuclio.Logger),
		config:  config,
		options: options,
	}

	// set cmdrunner
	env.cmdRunner, err = cmdrunner.NewCmdRunner(env.logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create command runner")
	}

	// generate an output name from the options
	env.outputName = env.getOutputName()

	if err = env.init(); err != nil {
		return nil, errors.Wrap(err, "Failed to init env")
	}

	return env, nil
}

func (e *env) getUserFunctionPath() string {
	return e.userFunctionPath
}

func (e *env) getWorkDir() string {
	return e.workDir
}

func (e *env) getNuclioDir() string {
	return e.nuclioDestDir
}

func (e *env) getBinaryPath() string {
	return filepath.Join(e.workDir, "processor")
}

func (e *env) getOutputName() string {
	if e.options.OutputName == "" {
		if e.options.OutputType == "docker" {
			return fmt.Sprintf("nuclio_processor_%s:%s", e.config.Name, e.options.Version)
		} else if e.options.OutputType == "binary" {
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Sprintf("nuclio_processor_%s_%s", e.config.Name, e.options.Version)
			} else {
				return filepath.Join(dir, fmt.Sprintf("nuclio_processor_%s_%s", e.config.Name, e.options.Version))
			}
		}
	} else {
		if e.options.OutputType == "docker" && !strings.ContainsRune(e.options.OutputName, ':') {
			return fmt.Sprintf("%s:%s", e.options.OutputName, e.options.Version)
		} else if e.options.OutputType == "binary" {
			return fmt.Sprintf("%s_%s", e.options.OutputName, e.options.Version)
		}
	}

	return e.options.OutputName
}

func (e *env) getNuclioSource() error {

	if e.options.NuclioSourceDir == "" {
		url, ref := e.parseGitUrl(e.options.NuclioSourceURL)

		_, err := e.cmdRunner.Run(nil, "git clone %s %s", url, e.nuclioDestDir)
		if err != nil {
			return errors.Wrap(err, "Unable to clone nuclio")
		}

		if ref != nil {
			workingDir := e.nuclioDestDir
			_, err := e.cmdRunner.Run(&cmdrunner.RunOptions{WorkingDir: &workingDir}, "git checkout %s", *ref)

			if err != nil {
				return errors.Wrapf(err, "Unable to checkout nuclio ref %s", *ref)
			}
		}
	} else {
		_, err := e.cmdRunner.Run(nil, "cp -R %s %s", e.options.NuclioSourceDir, e.nuclioDestDir)
		if err != nil {
			return errors.Wrap(err, "Unable to copy nuclio from local directory")
		}
	}

	e.logger.Debug("Completed getting nuclio source")
	return nil
}

func (e *env) parseGitUrl(url string) (string, *string) {
	urlAndRef := strings.Split(url, "#")
	if len(urlAndRef) == 2 {
		return urlAndRef[0], &urlAndRef[1]
	}

	return url, nil
}

func (e *env) writeRegistryFile(path string, env *env) error {
	t, err := template.New("registry").Parse(registryFileTemplate)

	if err != nil {
		return errors.Wrap(err, "Unable to create registry template.")
	}

	registryFilePath := filepath.Join(path, "nuclio_user_functions__"+strings.ToLower(e.config.Name)+".go")
	e.logger.DebugWith("Writing registry file", "path", registryFilePath)

	var buffer bytes.Buffer
	if err := t.Execute(&buffer, *env.config); err != nil {
		return err
	}

	return ioutil.WriteFile(registryFilePath, buffer.Bytes(), 0644)
}

func (e *env) ExternalConfigFilePaths() []string {
	var files []string
	processorConfigFilePath := filepath.Join(e.options.FunctionPath, processorConfigFileName)

	processorConfig := viper.New()
	processorConfig.SetConfigFile(processorConfigFilePath)
	if err := processorConfig.ReadInConfig(); err != nil {
		return files
	}

	for _, key := range processorConfig.AllKeys() {
		if strings.HasSuffix(key, ".config_path") {
			files = append(files, processorConfig.GetString(key))
		}
	}

	return files
}

func (e *env) isGoRuntime() bool {
	processorConfigFilePath := filepath.Join(e.options.FunctionPath, processorConfigFileName)
	processorConfig, err := processorconfig.ReadProcessorConfiguration(processorConfigFilePath)
	if err != nil {
		e.logger.DebugWith("Can't read processor configuration file", "error", err)
		return true
	}

	functionSection := processorConfig["function"]
	if functionSection == nil {
		return true
	}

	kind := functionSection.GetString("kind")
	if len(kind) == 0 {
		return true
	}

	return kind == "go"
}

func (e *env) createUserFunctionPath() error {
	e.userFunctionPath = filepath.Join(append([]string{e.nuclioDestDir}, userFunctionPath...)...)
	e.logger.DebugWith("Creating user function path", "path", e.userFunctionPath)

	fi, err := os.Stat(e.nuclioDestDir)
	if err != nil {
		return errors.Wrap(err, "can't get nuclio's directory information.")
	}

	if err := os.MkdirAll(e.userFunctionPath, fi.Mode()); err != nil {
		return errors.Wrapf(err, "error creating %s.", e.userFunctionPath)
	}

	copyFrom := e.options.getFunctionDir()

	functionCompletePath := filepath.Join(e.userFunctionPath, e.config.Name)
	e.logger.DebugWith("Copying user data", "from", copyFrom, "to", functionCompletePath)
	ok, err := util.CopyDir(copyFrom, functionCompletePath)
	if err != nil {
		return errors.Wrapf(err, "Error copying from %s to %s.", copyFrom, functionCompletePath)
	}
	if !ok {
		e.logger.DebugWith("No data copied")
	}

	// check if processor.yaml not provided. if it isn't, create it because docker COPYs this in
	// and it must exist
	processorConfigFilePath := filepath.Join(functionCompletePath, processorConfigFileName)
	if _, err := os.Stat(processorConfigFilePath); os.IsNotExist(err) {

		// create the file
		file, err := os.OpenFile(processorConfigFilePath, os.O_RDONLY|os.O_CREATE, 0666)
		if err != nil {
			return errors.Wrap(err, "Failed to create default processor config file")
		}

		// close the file, don't wait for garbage collection
		file.Close()

		e.logger.DebugWith("Processor config doesn't exist. Creating", "path", processorConfigFilePath)
	}

	registryPath := filepath.Join(append([]string{e.nuclioDestDir}, userFunctionRegistryPath...)...)

	return e.writeRegistryFile(registryPath, e)
}

func (e *env) init() error {
	tempDir, err := ioutil.TempDir("", "nuclio")

	if err != nil {
		return errors.Wrap(err, "Unable to create temp working directory")
	}

	e.workDir = tempDir
	e.nuclioDestDir = filepath.Join(tempDir, "nuclio")

	e.logger.DebugWith("Initializing", "work_dir", e.workDir, "dest_dir", e.nuclioDestDir)
	if err := e.getNuclioSource(); err != nil {
		return err
	}

	if e.isGoRuntime() {
		if err := e.createUserFunctionPath(); err != nil {
			return err
		}
	}
	return nil
}
