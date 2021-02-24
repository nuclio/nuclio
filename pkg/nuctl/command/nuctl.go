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

package command

import (
	"os"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/loggerus"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/factory"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	// load authentication modes
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

type RootCommandeer struct {
	loggerInstance          logger.Logger
	cmd                     *cobra.Command
	platformName            string
	platform                platform.Platform
	namespace               string
	verbose                 bool
	noColor                 bool
	loggerFormatterKind     string
	loggerFileFormatterKind string
	loggerFilePath          string
	KubeconfigPath          string

	platformConfiguration *platformconfig.Config
}

func NewRootCommandeer() *RootCommandeer {
	commandeer := &RootCommandeer{}

	cmd := &cobra.Command{
		Use:           "nuctl [command]",
		Short:         "Nuclio command-line interface",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	defaultPlatformType := common.GetEnvOrDefaultString("NUCTL_PLATFORM", "auto")
	defaultNamespace := os.Getenv("NUCTL_NAMESPACE")

	cmd.PersistentFlags().StringVar(&commandeer.loggerFilePath, "logger-file-path", "", "If passed, logger outputs to a file as well")
	cmd.PersistentFlags().BoolVar(&commandeer.noColor, "logger-no-color", false, "If passed, logger does not output with colors")
	cmd.PersistentFlags().StringVar(&commandeer.loggerFormatterKind, "logger-formatter-kind", "text", `Log formatter to use - "text" or "json" (Default: text)`)
	cmd.PersistentFlags().StringVar(&commandeer.loggerFileFormatterKind, "logger-file-formatter-kind", "json", `Log formatter to use - "text" or "json" (Default: json, used in conjunction with 'logger-file-path')`)
	cmd.PersistentFlags().BoolVarP(&commandeer.verbose, "verbose", "v", false, "Verbose output")
	cmd.PersistentFlags().StringVarP(&commandeer.platformName, "platform", "", defaultPlatformType, "Platform identifier - \"kube\", \"local\", or \"auto\"")
	cmd.PersistentFlags().StringVarP(&commandeer.namespace, "namespace", "n", defaultNamespace, "Namespace")

	// add children
	cmd.AddCommand(
		newBuildCommandeer(commandeer).cmd,
		newDeployCommandeer(commandeer).cmd,
		newInvokeCommandeer(commandeer).cmd,
		newGetCommandeer(commandeer).cmd,
		newDeleteCommandeer(commandeer).cmd,
		newUpdateCommandeer(commandeer).cmd,
		newVersionCommandeer(commandeer).cmd,
		newCreateCommandeer(commandeer).cmd,
		newExportCommandeer(commandeer).cmd,
		newImportCommandeer(commandeer).cmd,
	)

	commandeer.cmd = cmd

	return commandeer
}

// Execute uses os.Args to execute the command
func (rc *RootCommandeer) Execute() error {
	return rc.cmd.Execute()
}

// GetCmd returns the underlying cobra command
func (rc *RootCommandeer) GetCmd() *cobra.Command {
	return rc.cmd
}

// CreateMarkdown generates MD files in the target path
func (rc *RootCommandeer) CreateMarkdown(path string) error {
	return doc.GenMarkdownTree(rc.cmd, path)
}

func (rc *RootCommandeer) initialize() error {
	var err error

	rc.loggerInstance, err = rc.createLogger()
	if err != nil {
		return errors.Wrap(err, "Failed to create logger")
	}

	// TODO: accept platform config path as arg
	rc.platformConfiguration, err = platformconfig.NewPlatformConfig("")
	if err != nil {
		return errors.Wrap(err, "Failed to create platform config")
	}

	rc.platformConfiguration.Kube.KubeConfigPath = rc.KubeconfigPath

	// do not let nuctl monitor function containers
	// nuctl is a CLI tool, to enable function container healthiness, use Nuclio dashboard
	rc.platformConfiguration.Local.FunctionContainersHealthinessEnabled = false

	// ask the factory to create the appropriate platform
	// TODO: as more platforms are supported, i imagine the last argument will be to some
	// sort of configuration provider interface
	rc.platform, err = factory.CreatePlatform(rc.loggerInstance, rc.platformName, rc.platformConfiguration, rc.namespace)
	if err != nil {
		return errors.Wrap(err, "Failed to create platform")
	}

	// use default namespace by platform if specified
	if rc.namespace == "" {
		rc.namespace = rc.platform.ResolveDefaultNamespace(rc.namespace)
	}

	rc.loggerInstance.DebugWith("Created platform", "name", rc.platform.GetName())
	return nil
}

func (rc *RootCommandeer) createLogger() (logger.Logger, error) {
	var loggers []logger.Logger
	var loggerLevel logger.Level

	if rc.verbose {
		loggerLevel = logger.LevelDebug
	} else {
		loggerLevel = logger.LevelInfo
	}

	loggerInstance, err := loggerus.CreateStdoutLogger("nuctl",
		loggerLevel,
		loggerus.LoggerFormatterKind(rc.loggerFormatterKind),
		rc.noColor)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create stdout logger")
	}

	loggers = append(loggers, loggerInstance)

	if rc.loggerFilePath != "" {
		fileLoggerInstance, err := loggerus.CreateFileLogger("nuctl",
			loggerLevel,
			loggerus.LoggerFormatterKind(rc.loggerFileFormatterKind),
			rc.loggerFilePath,
			rc.noColor)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create file logger")
		}

		loggers = append(loggers, fileLoggerInstance)
	}

	return loggerus.MuxLoggers(loggers...)
}
