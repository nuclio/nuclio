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

package runtime

import (
	"net/url"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/nuclio-sdk"
	"github.com/v3io/v3io-go-http"
)

func newContext(parentLogger nuclio.Logger, configuration *Configuration) (*nuclio.Context, error) {

	newContext := &nuclio.Context{
		Logger:      parentLogger,
		DataBinding: map[string]nuclio.DataBinding{},
	}
	// create v3io context if applicable
	for dataBindingName, dataBinding := range configuration.Spec.DataBindings {
		if dataBinding.Class == "v3io" {

			// create a container object that can be used by the event handlers
			container, err := createV3ioDataBinding(parentLogger, dataBinding.URL)
			if err != nil {
				return nil, errors.Wrapf(err, "Failed to create v3io client for %s", dataBinding.URL)
			}

			newContext.DataBinding[dataBindingName] = container
		}
	}

	return newContext, nil
}

func createV3ioDataBinding(parentLogger nuclio.Logger, url string) (*v3io.Container, error) {
	parentLogger.InfoWith("Creating v3io data binding", "url", url)

	// parse the URL to get address and container ID
	addr, containerAlias, err := parseURL(url)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse URL")
	}

	// create context
	context, err := v3io.NewContext(parentLogger, addr, 8)
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

func parseURL(rawURL string) (addr string, containerAlias string, err error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		err = errors.Wrap(err, "Failed to parse URL")
		return
	}

	// get the container alias (at the very least /x (2 chars)
	if len(parsedURL.RequestURI()) < 2 {
		err = errors.New("Container alias missing in URL")
		return
	}

	containerAlias = parsedURL.RequestURI()[1:]
	addr = parsedURL.Host

	return
}
