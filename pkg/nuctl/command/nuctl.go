/*
Copyright 2023 The Nuclio Authors.

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
	"runtime"

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
	concurrency    int

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

	defaultPlatformType := common.GetEnvOrDefaultString("NUCTL_PLATFORM", common.AutoPlatformName)
	defaultNamespace := os.Getenv("NUCTL_NAMESPACE")
	ctx := context.Background()

	cmd.PersistentFlags().BoolVarP(&commandeer.verbose, "verbose", "v", false, "Verbose output")
	cmd.PersistentFlags().StringVarP(&commandeer.platformName, "platform", "", defaultPlatformType, "Platform identifier - \"kube\", \"local\", or \"auto\"")
	cmd.PersistentFlags().StringVarP(&commandeer.namespace, "namespace", "n", defaultNamespace, "Namespace")
	cmd.PersistentFlags().IntVar(&commandeer.concurrency, "concurrency", runtime.NumCPU(), "Max number of parallel patches. The default value is equal to the number of CPUs.")

	// platform specific
	cmd.PersistentFlags().StringVarP(&commandeer.KubeconfigPath, "kubeconfig", "k", "", "Path to a Kubernetes configuration file (admin.conf)")

	// add children
	cmd.AddCommand(
		newBuildCommandeer(commandeer).cmd,
		newDeployCommandeer(ctx, commandeer).cmd,
		newInvokeCommandeer(ctx, commandeer).cmd,
		newGetCommandeer(ctx, commandeer).cmd,
		newDeleteCommandeer(ctx, commandeer).cmd,
		newUpdateCommandeer(ctx, commandeer).cmd,
		newVersionCommandeer(commandeer).cmd,
		newCreateCommandeer(ctx, commandeer).cmd,
		newExportCommandeer(ctx, commandeer).cmd,
		newImportCommandeer(ctx, commandeer).cmd,
		newBetaCommandeer(ctx, commandeer).cmd,
		newParseCommandeer(ctx, commandeer).cmd,
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

func (rc *RootCommandeer) initialize(initPlatform bool) error {
	var err error

	rc.loggerInstance, err = rc.createLogger()
	if err != nil {
		return errors.Wrap(err, "Failed to create logger")
	}
	if !initPlatform {
		return nil
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

	// resolve namespace
	if err := rc.resolveDefaultNamespace(); err != nil {
		return errors.Wrap(err, "Failed to resolve default namespace")
	}

	// ask the factory to create the appropriate platform
	// TODO: as more platforms are supported, i imagine the last argument will be to some
	// sort of configuration provider interface
	rc.platform, err = factory.CreatePlatform(context.Background(),
		rc.loggerInstance,
		rc.platformName,
		rc.platformConfiguration,
		rc.namespace)
	if err != nil {
		return errors.Wrap(err, "Failed to create platform")
	}

	rc.loggerInstance.DebugWith("Created platform",
		"name", rc.platform.GetName(),
		"namespace", rc.namespace)
	return nil
}

func (rc *RootCommandeer) createLogger() (logger.Logger, error) {
	var loggerLevel nucliozap.Level

	if rc.verbose {
		loggerLevel = nucliozap.DebugLevel
	} else {
		loggerLevel = nucliozap.InfoLevel
	}

	loggerInstance, err := nucliozap.NewNuclioZapCmd("nuctl",
		loggerLevel,
		common.GetRedactorInstance(rc.GetCmd().OutOrStdout()))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create logger")
	}

	return loggerInstance, nil
}

func (rc *RootCommandeer) resolveDefaultNamespace() error {

	// if namespace is already set, use it
	if rc.namespace != "" {
		return nil
	}
	platformType, err := factory.GetPlatformByType(rc.platformName, rc.platformConfiguration)
	if err != nil {
		return errors.Wrap(err, "Failed to get platform by type")
	}

	switch platformType {
	case common.KubePlatformName:
		clientCmd, err := common.GetKubeConfigClientCmdByKubeconfigPath(common.GetKubeconfigPath(rc.KubeconfigPath))
		if err != nil {
			return errors.Wrap(err, "Failed to load kubeconfig")
		}
		if clientCmd.CurrentContext == "" {
			return errors.New("Failed to get current context - is your kubeconfig points to a valid cluster?")
		}
		rc.namespace = clientCmd.Contexts[clientCmd.CurrentContext].Namespace
	default:
		rc.namespace = "nuclio"
	}
	return nil
}
