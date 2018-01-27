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

package v3io

import (
	"net/url"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/databinding"

	"github.com/nuclio/logger"
	v3iohttp "github.com/v3io/v3io-go-http"
)

type v3io struct {
	databinding.AbstractDataBinding
	configuration *Configuration
	container     *v3iohttp.Container
}

func newDataBinding(parentLogger logger.Logger, configuration *Configuration) (databinding.DataBinding, error) {
	newV3io := v3io{
		AbstractDataBinding: databinding.AbstractDataBinding{
			Logger: parentLogger.GetChild("v3io"),
		},
		configuration: configuration,
	}

	newV3io.Logger.InfoWith("Creating", "configuration", configuration)

	return &newV3io, nil
}

// Start will start the data binding, connecting to the remote resource
func (v *v3io) Start() error {
	var err error

	v.Logger.InfoWith("Starting", "URL", v.configuration.URL)

	// try to create a container
	v.container, err = v.createContainer(v.Logger, v.configuration.URL)
	if err != nil {
		return errors.Wrap(err, "Failed to create v3io container")
	}

	return nil
}

// GetContextObject will return the object that is injected into the context
func (v *v3io) GetContextObject() (interface{}, error) {
	return v.container, nil
}

func (v *v3io) createContainer(parentLogger logger.Logger, url string) (*v3iohttp.Container, error) {
	parentLogger.InfoWith("Creating v3io data binding", "url", url)

	// parse the URL to get address and container ID
	addr, containerAlias, err := v.parseURL(url)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse URL")
	}

	// create context
	context, err := v3iohttp.NewContext(parentLogger, addr, 8)
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

func (v *v3io) parseURL(rawURL string) (addr string, containerAlias string, err error) {
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
