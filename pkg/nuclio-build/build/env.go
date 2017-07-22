package build

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/nuclio/nuclio/pkg/nuclio-build/util"
	"github.com/nuclio/nuclio/pkg/util/cmdrunner"

	"github.com/nuclio/nuclio-sdk"
	"github.com/pkg/errors"
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
			workingDir := "e.nuclioDestDir"
			_, err := e.cmdRunner.Run(&cmdrunner.RunOptions{WorkingDir: &workingDir}, "git checkout %s", ref)

			if err != nil {
				return errors.Wrap(err, "Unable to checkout nuclio ref")
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

	functionCompletePath := filepath.Join(e.userFunctionPath, e.config.Name)
	e.logger.DebugWith("Copying user data", "from", e.options.FunctionPath, "to", functionCompletePath)
	if err := util.CopyDir(e.options.FunctionPath, functionCompletePath); err != nil {
		return errors.Wrapf(err, "error when copying from %s to %s.", e.options.FunctionPath, functionCompletePath)
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

func (e *env) createDepsFile() error {
	if len(e.config.Build.Packages) > 0 {
		e.logger.DebugWith("Found packages to use as deps", "pkg", e.config.Build.Packages)
		var buffer bytes.Buffer

		for _, pack := range e.config.Build.Packages {
			if _, err := buffer.WriteString(pack); err != nil {
				return errors.Wrap(err, "Fail to write package data to buffer.")
			}
			if _, err := buffer.WriteRune('\n'); err != nil {
				return errors.Wrap(err, "Fail to write package data (new line) to buffer.")
			}
		}

		depsFilePath := filepath.Join(e.nuclioDestDir, ".deps")

		e.logger.DebugWith("Outputting deps file", "path", depsFilePath)
		if err := ioutil.WriteFile(depsFilePath, buffer.Bytes(), 0644); err != nil {
			return errors.Wrap(err, "Error outputing packages to .deps file.")
		}
	}
	return nil
}

func (e *env) init() error {
	tempDir, err := ioutil.TempDir("", "nuclio")

	if err != nil {
		return errors.Wrap(err, "Unable to create temp working directory")
	} else {
		e.workDir = tempDir
		e.nuclioDestDir = filepath.Join(tempDir, "nuclio")
	}

	e.logger.DebugWith("Initializing", "work_dir", e.workDir, "dest_dir", e.nuclioDestDir)
	for _, step := range []func() error{e.getNuclioSource, e.createUserFunctionPath, e.createDepsFile} {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}
