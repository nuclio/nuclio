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

package v3ioclient

import (
	"net/url"

	"github.com/nuclio/nuclio-sdk"

	"github.com/iguazio/v3io-go-http"
	"github.com/pkg/errors"
)

// thin wrapper for v3iow
type V3ioClient struct {
	*v3io.Container
}

func NewV3ioClient(parentLogger nuclio.Logger, url string) (*V3ioClient, error) {

	// parse the URL to get address and container ID
	addr, containerAlias, err := parseURL(url)

	// create client
	client, err := v3io.NewClient(parentLogger, addr)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create client")
	}

	// create session
	session, err := client.NewSession("", "", "nuclio")
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create session")
	}

	// create the container
	container, err := session.NewContainer(containerAlias)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create container")
	}

	newV3ioClient := &V3ioClient{
		Container: container,
	}

	return newV3ioClient, nil
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
