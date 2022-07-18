package kubectlclient

import (
	"github.com/nuclio/nuclio/pkg/cmdrunner"
)

type RunOptions struct {
	*cmdrunner.RunOptions
	KubectlRunOptions CommandExecutor
}

func NewRunOptions(kubectlRunOptions CommandExecutor) *RunOptions {
	runOptions := &RunOptions{
		KubectlRunOptions: kubectlRunOptions,
	}
	return runOptions
}

func WithMinikubeKubectlCommandRunner(profile string) *minikubeRunOptions {
	minikubeRunOptions := &minikubeRunOptions{
		Profile: profile,
	}
	return minikubeRunOptions
}

func WithDirectKubectlCommandRunner() *directRunOptions {
	return &directRunOptions{}
}

func RunKubectlCommand(cmdRunner cmdrunner.CmdRunner,
	positionalArgs []string,
	namedArgs map[string]string,
	runOptions *RunOptions) (cmdrunner.RunResult, error) {

	if runOptions == nil {
		runOptions = NewRunOptions(WithDirectKubectlCommandRunner())
	}

	var argsStringSlice []string
	argsStringSlice = append(argsStringSlice, runOptions.KubectlRunOptions.GetRunCommand())
	argsStringSlice = append(argsStringSlice, positionalArgs...)

	return cmdRunner.RunWithPositionalAndNamedArguments(runOptions.RunOptions, argsStringSlice, namedArgs)
}
