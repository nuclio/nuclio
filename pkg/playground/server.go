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

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/dockercreds"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/go-chi/chi"
	"github.com/go-chi/cors"
	"github.com/nuclio/nuclio-sdk"
)

type Server struct {
	*restful.Server
	assetsDir             string
	sourcesDir            string
	dockerKeyDir          string
	defaultRegistryURL    string
	defaultRunRegistryURL string
	dockerClient          dockerclient.Client
	dockerCreds           *dockercreds.DockerCreds
	Platform              platform.Platform
	NoPullBaseImages      bool
}

func NewServer(parentLogger nuclio.Logger,
	assetsDir string,
	sourcesDir string,
	dockerKeyDir string,
	defaultRegistryURL string,
	defaultRunRegistryURL string,
	platform platform.Platform,
	noPullBaseImages bool) (*Server, error) {

	var err error

	newDockerClient, err := dockerclient.NewShellClient(parentLogger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create docker client")
	}

	newDockerCreds, err := dockercreds.NewDockerCreds(parentLogger, newDockerClient)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create docker loginner")
	}

	newServer := &Server{
		assetsDir:             assetsDir,
		sourcesDir:            sourcesDir,
		dockerKeyDir:          dockerKeyDir,
		defaultRegistryURL:    defaultRegistryURL,
		defaultRunRegistryURL: defaultRunRegistryURL,
		dockerClient:          newDockerClient,
		dockerCreds:           newDockerCreds,
		Platform:              platform,
		NoPullBaseImages:      noPullBaseImages,
	}

	// create server
	newServer.Server, err = restful.NewServer(parentLogger, PlaygroundResourceRegistrySingleton, newServer)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create restful server")
	}

	// add static file patterns
	if err := newServer.addAssetRoutes(); err != nil {
		return nil, errors.Wrap(err, "Failed to add asset routes")
	}

	// try to load docker keys, ignoring errors
	if err := newServer.loadDockerKeys(newServer.dockerKeyDir); err != nil {
		newServer.Logger.WarnWith("Failed to login with docker keys", "err", err.Error())
	}

	newServer.Logger.InfoWith("Initialized",
		"assetsDir", assetsDir,
		"sourcesDir", sourcesDir,
		"dockerKeyDir", dockerKeyDir,
		"defaultRegistryURL", defaultRegistryURL,
		"defaultRunRegistryURL", defaultRunRegistryURL)

	return newServer, nil
}

func (s *Server) GetSourcesDir() string {
	return s.sourcesDir
}

func (s *Server) GetRegistryURL() string {
	return s.defaultRegistryURL
}

func (s *Server) GetRunRegistryURL() string {
	return s.defaultRunRegistryURL
}

func (s *Server) InstallMiddleware(router chi.Router) error {
	if err := s.Server.InstallMiddleware(router); err != nil {
		return err
	}

	cors := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-nuclio-log-level"},
		ExposedHeaders:   []string{"X-nuclio-logs"},
		AllowCredentials: true,
		MaxAge:           300,
	})

	router.Use(cors.Handler)

	return nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(201)
}

func (s *Server) addAssetRoutes() error {
	fileServer := http.FileServer(http.Dir(s.assetsDir))
	s.Router.Get("/assets/*", fileServer.ServeHTTP)

	// serve index.html
	for _, pattern := range []string{"/", "/index.htm", "/index.html"} {
		s.Router.Get(pattern, s.serveIndex)
	}

	return nil
}

func (s *Server) serveIndex(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "public, max-age=86400") // Timeout after 24 hours

	indexHTMLContents, err := ioutil.ReadFile(path.Join(s.assetsDir, "index.html"))
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	writer.Write(indexHTMLContents)
}

func (s *Server) loadDockerKeys(dockerKeyDir string) error {
	if dockerKeyDir == "" {
		return nil
	}

	return s.dockerCreds.LoadFromDir(dockerKeyDir)
}
