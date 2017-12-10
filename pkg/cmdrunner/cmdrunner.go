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

package cmdrunner

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"github.com/nuclio/nuclio-sdk"
)

type CaptureOutputModeType int

// RunOptions specifies options to CmdRunner.Run
type RunOptions struct {
	WorkingDir        *string
	Stdin             *string
	Env               map[string]string
	CaptureOutputMode CaptureOutputModeType
}

type RunResult struct {
	StdOut   string
	StdError string
	ExitCode int
}

// CmdRunner specifies the interface to an underlying command runner
type CmdRunner interface {
	// Run runs a command, given options
	Run(options *RunOptions, format string, vars ...interface{}) (RunResult, error)
}

type ShellRunner struct {
	logger nuclio.Logger
	shell  string
}

func NewShellRunner(parentLogger nuclio.Logger) (*ShellRunner, error) {
	return &ShellRunner{
		logger: parentLogger.GetChild("runner"),
		shell:  "/bin/sh",
	}, nil
}

func (sr *ShellRunner) Run(options *RunOptions, format string, vars ...interface{}) (RunResult, error) {

	// format the command
	command := fmt.Sprintf(format, vars...)
	sr.logger.DebugWith("Executing", "command", command, "options", options)

	// create a command
	cmd := exec.Command(sr.shell, "-c", command)

	// if there are options, set them
	if options != nil {
		if options.WorkingDir != nil {
			cmd.Dir = *options.WorkingDir
		}

		// get environment variables if any
		if options.Env != nil {
			cmd.Env = sr.getEnvFromOptions(options)
		}

		if options.Stdin != nil {
			cmd.Stdin = strings.NewReader(*options.Stdin)
		}
	}

	var stdOut, stdErr bytes.Buffer
	var output []byte
	var err error

	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	if err = cmd.Run(); err != nil {
		var exitCode int
		sr.logger.DebugWith("Failed to execute command", "output", string(output), "err", err)

		// Did the command fail because of an unsuccessful exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.Sys().(syscall.WaitStatus).ExitStatus()
		}

		return RunResult{stdOut.String(), stdErr.String(), exitCode}, err
	}

	stringOutput := string(output)

	sr.logger.DebugWith("Command executed successfully", "output", stringOutput)

	return RunResult{stdOut.String(), stdErr.String(), 0}, nil
}

func (sr *ShellRunner) SetShell(shell string) {
	sr.shell = shell
}

func (sr *ShellRunner) getEnvFromOptions(options *RunOptions) []string {
	envs := []string{}

	for name, value := range options.Env {
		envs = append(envs, fmt.Sprintf("%s=%s", name, value))
	}

	return envs
}
