//go:build test_unit || (test_integration && (test_kube || test_local || test_functions_kube))

/*
Copyright 2025 The Nuclio Authors.

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

package test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/nuctl/command"

	"github.com/nuclio/logger"
)

type NuctlRunner struct {
	outputBuffer bytes.Buffer
	stdinReader  *strings.Reader
	Namespace    string
	Logger       logger.Logger
}

func NewNuctlRunner(namespace string, logger logger.Logger) *NuctlRunner {
	return &NuctlRunner{
		Namespace: namespace,
		Logger:    logger.GetChild("nuctl.runner"),
	}
}

// Run executes nuctl command with the given positional and named arguments
func (nr *NuctlRunner) Run(positionalArgs []string,
	namedArgs map[string]string) error {

	// reset buffer to ensure it contains only last executed command
	nr.outputBuffer.Reset()
	rootCommandeer := command.NewRootCommandeer()

	// set the output, so we can capture it (but also output to stdout)
	rootCommandeer.GetCmd().SetOut(io.MultiWriter(os.Stdout, &nr.outputBuffer))

	// set the input so we can write to stdin
	if nr.stdinReader != nil {
		rootCommandeer.GetCmd().SetIn(nr.stdinReader)
	}

	var argsStringSlice []string

	// add positional arguments
	argsStringSlice = append(argsStringSlice, positionalArgs...)

	for argName, argValue := range namedArgs {
		argsStringSlice = append(argsStringSlice, fmt.Sprintf("--%s", argName), argValue)
	}

	if nr.isNamespaceRequired() && !nr.namespaceInArgs(positionalArgs, namedArgs) {
		// prepend namespace to args
		argsStringSlice = common.PrependStringsToStringSlice(argsStringSlice, "--namespace", nr.Namespace)
	}

	// since args[0] is the executable name, just shove the binary there
	argsStringSlice = common.PrependStringToStringSlice(argsStringSlice, "nuctl")

	// override os.Args to simulate command-line arguments in tests,
	// allowing Cobra to parse the nuctl command as if run from the terminal.
	// override is required because Cobra reads os.Args directly for command parsing.
	os.Args = argsStringSlice

	nr.Logger.InfoWith("Executing nuctl", "args", argsStringSlice)

	// execute
	return rootCommandeer.Execute()
}

func (nr *NuctlRunner) namespaceInArgs(positionalArgs []string, namedArgs map[string]string) bool {
	if common.StringSliceContainsString(positionalArgs, "--namespace") || common.StringSliceContainsString(positionalArgs, "-n") {
		return true
	}

	if _, ok := namedArgs["namespace"]; ok {
		return true
	}

	return false
}

func (nr *NuctlRunner) isNamespaceRequired() bool {
	return nr.Namespace != "" && common.GetKubeconfigPath("") != ""
}
