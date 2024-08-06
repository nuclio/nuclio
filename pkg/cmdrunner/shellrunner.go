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

package cmdrunner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path"
	"strings"
	"syscall"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type ShellRunner struct {
	logger logger.Logger
	shell  string
}

func NewShellRunner(parentLogger logger.Logger) (*ShellRunner, error) {
	return &ShellRunner{
		logger: parentLogger.GetChild("runner"),
		shell:  "/bin/sh",
	}, nil
}

func (sr *ShellRunner) RunWithPositionalAndNamedArguments(runOptions *RunOptions,
	positionalArgs []string,
	namedArgs map[string]string) (RunResult, error) {

	argsStringSlice := make([]string, len(positionalArgs))
	copy(argsStringSlice, positionalArgs)

	for argName, argValue := range namedArgs {
		argsStringSlice = append(argsStringSlice, fmt.Sprintf("--%s %s", argName, argValue))
	}

	encodedCommand := strings.Join(argsStringSlice, " ")

	sr.logger.DebugWith("Running command", "encodedCommand", encodedCommand)
	return sr.Run(runOptions, encodedCommand)
}

func (sr *ShellRunner) Run(runOptions *RunOptions, format string, vars ...interface{}) (RunResult, error) {

	// support missing runOptions for tests that send nil
	if runOptions == nil {
		runOptions = &RunOptions{}
	}

	// format the command
	formattedCommand := fmt.Sprintf(format, vars...)
	redactedCommand := Redact(runOptions.LogRedactions, formattedCommand)

	if !runOptions.LogOnlyOnFailure {
		sr.logger.DebugWith("Executing", "command", redactedCommand)
	}

	// create a command
	cmd := exec.Command(sr.shell, "-c", formattedCommand)

	// if there are runOptions, set them
	if runOptions.WorkingDir != nil {
		cmd.Dir = *runOptions.WorkingDir
	}

	// get environment variables if any
	if runOptions.Env != nil {
		cmd.Env = sr.getEnvFromOptions(runOptions)
	}

	if runOptions.Stdin != nil {
		cmd.Stdin = strings.NewReader(*runOptions.Stdin)
	}

	runResult := RunResult{
		ExitCode: 0,
	}

	if err := sr.runAndCaptureOutput(cmd, runOptions, &runResult); err != nil {
		var exitCode int

		// Did the command fail because of an unsuccessful exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.Sys().(syscall.WaitStatus).ExitStatus()
		}

		runResult.ExitCode = exitCode
		if !runOptions.SkipLogOnFailure {
			sr.logger.DebugWith("Failed to execute command",
				"output", runResult.Output,
				"stderr", runResult.Stderr,
				"exitCode", runResult.ExitCode,
				"err", err)
		}

		return runResult, errors.Wrapf(err, "stdout:\n%s\nstderr:\n%s", runResult.Output, runResult.Stderr)
	}

	if !runOptions.LogOnlyOnFailure {
		sr.logger.DebugWith("Command executed successfully",
			"output", runResult.Output,
			"stderr", runResult.Stderr,
			"exitCode", runResult.ExitCode)
	}

	return runResult, nil
}

// CopyObjectsToContainer copies objects (files, directories) from a local storage to a container
// objectToCopy is a map where keys are local storage path and values are container paths
func (sr *ShellRunner) CopyObjectsToContainer(containerName string, objectsToCopy map[string]string) error {

	// copy objects
	for objectLocalPath, objectContainerPath := range objectsToCopy {

		// create target directory if it doesn't exist
		fileDir := path.Dir(objectContainerPath)
		if _, err := sr.Run(nil, "docker exec %s mkdir -p %s", containerName, fileDir); err != nil {
			return errors.Wrapf(err, "Failed creating directory in container")
		}

		// copy an object from local storage to the given container
		if _, err := sr.Run(nil,
			"docker cp %s %s:%s ",
			objectLocalPath,
			containerName,
			objectContainerPath); err != nil {
			return errors.Wrapf(err, "Failed copying object %s to container %s:%s", objectLocalPath, containerName, objectContainerPath)
		}
	}

	return nil
}

func (sr *ShellRunner) Stream(ctx context.Context,
	runOptions *RunOptions,
	format string,
	vars ...interface{}) (io.ReadCloser, error) {

	// support missing runOptions for tests that send nil
	if runOptions == nil {
		runOptions = &RunOptions{}
	}

	// format the command
	formattedCommand := fmt.Sprintf(format, vars...)
	redactedCommand := Redact(runOptions.LogRedactions, formattedCommand)

	if !runOptions.LogOnlyOnFailure {
		sr.logger.DebugWith("Executing", "command", redactedCommand)
	}

	// create a command
	cmd := exec.CommandContext(ctx, sr.shell, "-c", formattedCommand)

	// if there are runOptions, set them
	if runOptions.WorkingDir != nil {
		cmd.Dir = *runOptions.WorkingDir
	}

	// get environment variables if any
	if runOptions.Env != nil {
		cmd.Env = sr.getEnvFromOptions(runOptions)
	}

	if runOptions.Stdin != nil {
		cmd.Stdin = strings.NewReader(*runOptions.Stdin)
	}

	// Use stdout for standard error
	cmd.Stderr = cmd.Stdout

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create stdout pipe")
	}

	// start command
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "Failed to start command")
	}

	go func() {
		<-ctx.Done()
		if err := cmd.Wait(); err != nil {
			sr.logger.DebugWith("Stream command finished with an error", "err", err)
			return
		}
		sr.logger.Debug("Stream command finished")

	}()

	return stdoutPipe, nil
}

func (sr *ShellRunner) SetShell(shell string) {
	sr.shell = shell
}

func (sr *ShellRunner) getEnvFromOptions(runOptions *RunOptions) []string {
	envs := []string{}

	for name, value := range runOptions.Env {
		envs = append(envs, fmt.Sprintf("%s=%s", name, value))
	}

	return envs
}

func (sr *ShellRunner) runAndCaptureOutput(cmd *exec.Cmd,
	runOptions *RunOptions,
	runResult *RunResult) error {

	switch runOptions.CaptureOutputMode {

	case CaptureOutputModeCombined:
		stdoutAndStderr, err := cmd.CombinedOutput()
		runResult.Output = Redact(runOptions.LogRedactions, string(stdoutAndStderr))
		return err

	case CaptureOutputModeStdout:
		var stdOut, stdErr bytes.Buffer
		cmd.Stdout = &stdOut
		cmd.Stderr = &stdErr

		err := cmd.Run()

		runResult.Output = Redact(runOptions.LogRedactions, stdOut.String())
		runResult.Stderr = Redact(runOptions.LogRedactions, stdErr.String())

		return err
	}

	return fmt.Errorf("Invalid output capture mode: %d", runOptions.CaptureOutputMode)
}

func Redact(redactions []string, runOutput string) string {
	if redactions == nil {
		return runOutput
	}

	var replacements []string

	for _, redactionField := range redactions {
		replacements = append(replacements, redactionField, "[redacted]")
	}

	replacer := strings.NewReplacer(replacements...)
	return replacer.Replace(runOutput)
}
