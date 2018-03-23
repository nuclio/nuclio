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

package v3ioutil

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/logger"
	v3iohttp "github.com/v3io/v3io-go-http"
)

func CreateContainer(parentLogger logger.Logger,
	addr string,
	containerAlias string,
	numWorkers int) (*v3iohttp.Container, error) {
	parentLogger.InfoWith("Creating v3io data binding",
		"addr", addr,
		"containerAlias", containerAlias,
		"numWorkers", numWorkers)

	// create context
	context, err := v3iohttp.NewContext(parentLogger, addr, numWorkers)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create client")
	}

	// create session
	session, err := context.NewSession("", "", "nuclio")
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create session")
	}

	// create the container
	container, err := session.NewContainer(containerAlias)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create container")
	}

	return container, nil
}

func ParseURL(rawURL string) (addr string, containerAlias string, path string, err error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		err = errors.Wrapf(err, "Failed to parse URL: %s", rawURL)
		return
	}

	addr = parsedURL.Host

	// get the container alias (at the very least /x (2 chars)
	if len(parsedURL.RequestURI()) < 2 {
		err = fmt.Errorf("Missing container alias: %s", rawURL)
		return
	}

	// request URI holds container alias and path. trim slashes from both ends
	containerAliasAndPathString := strings.Trim(parsedURL.RequestURI(), "/")

	// split @ / - if there's a path, there should be two parts
	containerAliasAndPath := strings.SplitN(containerAliasAndPathString, "/", 2)

	switch len(containerAliasAndPath) {
	case 2:
		containerAlias = containerAliasAndPath[0]
		path = containerAliasAndPath[1]

	case 1:
		containerAlias = containerAliasAndPath[0]

	case 0:
		err = fmt.Errorf("Expected at least one part in request URI: %s", containerAliasAndPathString)
		return
	}

	return
}
