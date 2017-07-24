package cmdrunner

import (
	"fmt"
	"os/exec"

	"github.com/nuclio/nuclio-sdk"
	"strings"
)

type CmdRunner struct {
	logger nuclio.Logger
	shell  string
}

type RunOptions struct {
	WorkingDir *string
	Stdin      *string
	Env        map[string]string
}

func NewCmdRunner(parentLogger nuclio.Logger) (*CmdRunner, error) {
	return &CmdRunner{
		logger: parentLogger.GetChild("runner").(nuclio.Logger),
		shell:  "/bin/sh",
	}, nil
}

func (cr *CmdRunner) Run(options *RunOptions, format string, vars ...interface{}) (string, error) {

	// format the command
	command := fmt.Sprintf(format, vars...)
	cr.logger.DebugWith("Executing", "command", command, "options", options)

	// create a command
	cmd := exec.Command(cr.shell, "-c", command)

	// if there are options, set them
	if options != nil {
		if options.WorkingDir != nil {
			cmd.Dir = *options.WorkingDir
		}

		// get environment variables if any
		if options.Env != nil {
			cmd.Env = cr.getEnvFromOptions(options)
		}

		if options.Stdin != nil {
			cmd.Stdin = strings.NewReader(*options.Stdin)
		}
	}

	// run
	output, err := cmd.CombinedOutput()
	if err != nil {
		cr.logger.DebugWith("Failed to execute command", "err", err)
		return "", err
	}

	stringOutput := string(output)

	cr.logger.DebugWith("Command executed successfully", "output", stringOutput)

	return stringOutput, nil
}

func (cr *CmdRunner) SetShell(shell string) {
	cr.shell = shell
}

func (cr *CmdRunner) getEnvFromOptions(options *RunOptions) []string {
	envs := []string{}

	for name, value := range options.Env {
		envs = append(envs, fmt.Sprintf("%s=%s", name, value))
	}

	return envs
}
