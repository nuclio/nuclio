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
