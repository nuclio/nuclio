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

package playground

import (
	"io/ioutil"
	"net/http"
	"path"

	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/errors"
)

type Server struct {
	*restful.Server
	assetsDir string
}

func NewServer(parentLogger nuclio.Logger, assetsDir string) (*Server, error) {
	var err error

	newServer := &Server{
		assetsDir: assetsDir,
	}

	// create server
	newServer.Server, err = restful.NewServer(parentLogger, PlaygroundResourceRegistrySingleton, newServer)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create restful server")
	}

	// server index.html
	for _, pattern := range []string{"/", "/index.htm", "/index.html"} {
		newServer.Router.Get(pattern, newServer.serveIndex)
	}

	return newServer, nil
}

func (s *Server) serveIndex(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")

	indexHtmlContents, err := ioutil.ReadFile(path.Join(s.assetsDir, "index.html"))
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	writer.Write(indexHtmlContents)
}
