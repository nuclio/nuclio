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

package resource

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/playground"
	"github.com/nuclio/nuclio/pkg/restful"
)

type tunnelResource struct {
	*resource
}

// called after initialization
func (tr *tunnelResource) OnAfterInitialize() error {

	// all methods
	for _, registrar := range []func(string, http.HandlerFunc){
		tr.GetRouter().Get,
		tr.GetRouter().Post,
		tr.GetRouter().Put,
		tr.GetRouter().Delete,
		tr.GetRouter().Patch,
		tr.GetRouter().Options,
	} {
		registrar("/*", tr.handleRequest)
	}

	return nil
}

func (tr *tunnelResource) handleRequest(responseWriter http.ResponseWriter, request *http.Request) {

	// get host and URL from request
	host, path, err := tr.getTunneledHostAndPath(request.URL.Path)
	if err != nil {
		responseWriter.Write([]byte(`{"error": Invalid path - expected /tunnel/<host>/<path>""}`)) // nolint: errcheck
		responseWriter.WriteHeader(http.StatusBadRequest)
		return
	}

	tr.Logger.DebugWith("Tunneling request",
		"url", request.URL.Path,
		"host", host,
		"path", path)

	// create a url from the path
	fullURL := fmt.Sprintf("http://%s%s", host, path)

	// create a request
	tunneledRequest, err := http.NewRequest(request.Method, fullURL, request.Body)
	if err != nil {
		tr.Logger.WarnWith("Failed to create tunneled request", "err", err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	// set headers
	tunneledRequest.Header = request.Header

	// do the tunneling
	tunneledHTTPResponse, err := http.DefaultClient.Do(tunneledRequest)
	if err != nil {
		tr.Logger.WarnWith("Failed to tunnel request", "err", err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	responseBody, err := ioutil.ReadAll(tunneledHTTPResponse.Body)
	if err != nil {
		tr.Logger.WarnWith("Failed to read tunneled response", "err", err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	// set headers
	for headerName, headerValue := range tunneledHTTPResponse.Header {
		responseWriter.Header()[headerName] = headerValue
	}

	responseWriter.WriteHeader(tunneledHTTPResponse.StatusCode)
	responseWriter.Write(responseBody) // nolint: errcheck
}

func (tr *tunnelResource) getTunneledHostAndPath(fullPath string) (host string, path string, err error) {
	pathPrefix := "/tunnel"

	if len(fullPath) <= len(pathPrefix) {
		err = errors.New("Path is too short")
		return
	}

	// remove /tunnel portion and /
	fullPath = fullPath[len(pathPrefix)+1:]

	// split host portion
	hostAndPath := strings.SplitN(fullPath, "/", 2)

	// check if there are two sections
	if len(hostAndPath) != 2 {
		err = errors.New("Path is too short")
		return
	}

	host, path = hostAndPath[0], hostAndPath[1]

	// hack for local environments - if the user specifies "localhost", the invoke will not
	// work seeing how playground and function each have their own localhost interface. as such
	// try to work around this by using the docker interface IP.
	if strings.HasPrefix(host, "localhost") {
		host = strings.Replace(host, "localhost", "172.17.0.1", -1)
	}

	if strings.HasPrefix(host, "127.0.0.1") {
		host = strings.Replace(host, "127.0.0.1", "172.17.0.1", -1)
	}

	// add prefix to path
	path = "/" + path

	return
}

// register the resource
var tunnelResourceInstance = &tunnelResource{
	resource: newResource("tunnel", []restful.ResourceMethod{}),
}

func init() {
	tunnelResourceInstance.Resource = tunnelResourceInstance
	tunnelResourceInstance.Register(playground.PlaygroundResourceRegistrySingleton)
}
