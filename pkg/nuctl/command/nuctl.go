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
	"context"
	"os"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/factory"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	// load authentication modes
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

type RootCommandeer struct {
	loggerInstance logger.Logger
	cmd            *cobra.Command
	platformName   string
	platform       platform.Platform
	namespace      string
	verbose        bool
	KubeconfigPath string

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

	cmd.PersistentFlags().BoolVarP(&commandeer.verbose, "verbose", "v", false, "Verbose output")
	cmd.PersistentFlags().StringVarP(&commandeer.platformName, "platform", "", defaultPlatformType, "Platform identifier - \"kube\", \"local\", or \"auto\"")
	cmd.PersistentFlags().StringVarP(&commandeer.namespace, "namespace", "n", defaultNamespace, "Namespace")

	// platform specific
	cmd.PersistentFlags().StringVarP(&commandeer.KubeconfigPath, "kubeconfig", "k", "", "Path to a Kubernetes configuration file (admin.conf)")

	// add children
	cmd.AddCommand(
		newBuildCommandeer(commandeer).cmd,
		newDeployCommandeer(context.Background(), commandeer).cmd,
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
	rc.platform, err = factory.CreatePlatform(context.Background(), rc.loggerInstance, rc.platformName, rc.platformConfiguration, rc.namespace)
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
	var loggerLevel nucliozap.Level

	if rc.verbose {
		loggerLevel = nucliozap.DebugLevel
	} else {
		loggerLevel = nucliozap.InfoLevel
	}

	loggerInstance, err := nucliozap.NewNuclioZapCmd("nuctl", loggerLevel)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create logger")
	}

	return loggerInstance, nil
}
