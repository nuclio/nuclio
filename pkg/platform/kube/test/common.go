package test

import (
	"fmt"
	"strings"

	"github.com/nuclio/nuclio/pkg/cmdrunner"

	"github.com/nuclio/logger"
)

type runKubectlCommandMode string

const (
	runKubectlCommandDirect   runKubectlCommandMode = "direct"
	runKubectlCommandMinikube runKubectlCommandMode = "minikube"
)

func runKubectlCommand(logger logger.Logger,
	cmdrunner cmdrunner.CmdRunner,
	positionalArgs []string,
	namedArgs map[string]string,
	mode runKubectlCommandMode,
	runOptions *cmdrunner.RunOptions) (cmdrunner.RunResult, error) {

	var argsStringSlice []string

	switch mode {
	case runKubectlCommandDirect:
		argsStringSlice = append(argsStringSlice, "kubectl")
	case runKubectlCommandMinikube:
		argsStringSlice = append(argsStringSlice, "minikube kubectl --")
	default:
		argsStringSlice = append(argsStringSlice, "kubectl")
	}

	// add positional arguments
	argsStringSlice = append(argsStringSlice, positionalArgs...)

	return runCommand(logger, cmdrunner, argsStringSlice, namedArgs, runOptions)
}

func runCommand(logger logger.Logger,
	cmdrunner cmdrunner.CmdRunner,
	positionalArgs []string,
	namedArgs map[string]string,
	runOptions *cmdrunner.RunOptions) (cmdrunner.RunResult, error) {

	var argsStringSlice []string
	copy(argsStringSlice, positionalArgs)

	for argName, argValue := range namedArgs {
		argsStringSlice = append(argsStringSlice, fmt.Sprintf("--%s %s", argName, argValue))
	}

	encodedCommand := strings.Join(argsStringSlice, " ")

	logger.DebugWith("Running kubectl", "encodedCommand", encodedCommand)
	return cmdrunner.Run(runOptions, encodedCommand)

}
