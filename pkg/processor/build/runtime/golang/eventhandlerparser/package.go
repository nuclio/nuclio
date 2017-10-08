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

package eventhandlerparser

import (
	"bufio"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/nuclio-sdk"
)

const (
	handlerRegExp = "(?m)^\\s*func ([A-Z]\\w+)\\(context \\*nuclio.Context, event nuclio.Event\\) \\(interface{}, error\\)"
)

// PackageHandlerParser parsers event handlers in a package
type PackageHandlerParser struct {
	cmd       *cmdrunner.CmdRunner
	handlerRe *regexp.Regexp
	runOpts   *cmdrunner.RunOptions
}

// NewPackageHandlerParser returns new EventHandlerParser
func NewPackageHandlerParser(logger nuclio.Logger) (*PackageHandlerParser, error) {
	cmd, err := cmdrunner.NewCmdRunner(logger)
	if err != nil {
		return nil, err
	}

	re, err := regexp.Compile(handlerRegExp)
	if err != nil {
		return nil, err
	}

	tmpDirPath, err := ioutil.TempDir("", "package-parser")
	if err != nil {
		return nil, err
	}

	overrides := map[string]string{
		"GOPATH": tmpDirPath,
	}
	runOpts := &cmdrunner.RunOptions{Env: cmdrunner.OverrideEnv(overrides)}

	return &PackageHandlerParser{cmd, re, runOpts}, nil
}

// ParseEventHandlers return list of Nuclio event handlers in package
func (p *PackageHandlerParser) ParseEventHandlers(packageName string) ([]string, []string, error) {
	_, err := p.cmd.Run(p.runOpts, "go get %s", packageName)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Can't go get %q", packageName)
	}

	out, err := p.cmd.Run(p.runOpts, "go doc %s", packageName)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Can't go doc %q", packageName)
	}

	var handlers []string
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		fields := p.handlerRe.FindStringSubmatch(scanner.Text())
		if fields == nil {
			continue
		}
		handlers = append(handlers, fields[1])
	}

	if err = scanner.Err(); err != nil {
		return nil, nil, errors.Wrap(err, "Can't scan output")
	}

	return []string{packageName}, handlers, nil
}
