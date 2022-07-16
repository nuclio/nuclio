package test

import (
	"fmt"
	"strings"

	"github.com/nuclio/nuclio/pkg/cmdrunner"

	"github.com/nuclio/logger"
)

type RunKubectlCommandMode string

const (
	RunKubectlCommandDirect   RunKubectlCommandMode = "direct"
	RunKubectlCommandMinikube RunKubectlCommandMode = "minikube"
)

type RunOptions struct {
	*cmdrunner.RunOptions
	mode         RunKubectlCommandMode
	modeExecutor string
}

func NewRunOptions(mode RunKubectlCommandMode, modeExecutor string) *RunOptions {
	runOptions := &RunOptions{
		mode:         mode,
		modeExecutor: modeExecutor,
	}
	return runOptions
}

func RunKubectlCommand(logger logger.Logger,
	cmdrunner cmdrunner.CmdRunner,
	positionalArgs []string,
	namedArgs map[string]string,
	runOptions *RunOptions) (cmdrunner.RunResult, error) {

	if runOptions == nil {
		runOptions = NewRunOptions(RunKubectlCommandDirect, "kubectl")
	}

	var argsStringSlice []string

	switch runOptions.mode {
	case RunKubectlCommandDirect:
		argsStringSlice = append(argsStringSlice, runOptions.modeExecutor)
	case RunKubectlCommandMinikube:
		argsStringSlice = append(argsStringSlice, runOptions.modeExecutor)
	default:
		argsStringSlice = append(argsStringSlice, runOptions.modeExecutor)
	}

	// add positional arguments
	argsStringSlice = append(argsStringSlice, positionalArgs...)

	return RunCommand(logger, cmdrunner, argsStringSlice, namedArgs, runOptions.RunOptions)
}

func RunCommand(logger logger.Logger,
	cmdrunner cmdrunner.CmdRunner,
	positionalArgs []string,
	namedArgs map[string]string,
	runOptions *cmdrunner.RunOptions) (cmdrunner.RunResult, error) {

	argsStringSlice := make([]string, len(positionalArgs))
	copy(argsStringSlice, positionalArgs)

	for argName, argValue := range namedArgs {
		argsStringSlice = append(argsStringSlice, fmt.Sprintf("--%s %s", argName, argValue))
	}

	encodedCommand := strings.Join(argsStringSlice, " ")

	logger.DebugWith("Running command", "encodedCommand", encodedCommand)
	return cmdrunner.Run(runOptions, encodedCommand)
}
