/*
Copyright 2026 The Nuclio Authors.

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

package app

import (
	"fmt"
	"net/http"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/loggersink"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"

	// register logger sinks (stdout, appinsights) into the loggersink registry via init()
	_ "github.com/nuclio/nuclio/pkg/sinks"
)

// server fronts a function or the DLX.
type server struct {
	logger      logger.Logger
	httpServer  *http.Server
	upstreamURL string
	authURL     string
	redirectURL string
}

// Run creates and starts the auth-proxy server for the given mode.
func Run(mode auth.ProxyMode,
	listenPort int,
	upstreamURL string,
	authURL string,
	redirectURL string,
	platformConfigurationPath string) error {
	if err := validateConfiguration(listenPort, upstreamURL, authURL, redirectURL); err != nil {
		return errors.Wrap(err, "Invalid auth-proxy configuration")
	}

	platformConfiguration, err := platformconfig.NewPlatformConfig(platformConfigurationPath)
	if err != nil {
		return errors.Wrap(err, "Failed to get platform configuration")
	}

	rootLogger, err := loggersink.CreateSystemLogger("auth-proxy", platformConfiguration)
	if err != nil {
		return errors.Wrap(err, "Failed to create logger")
	}

	server, err := newServer(rootLogger, mode, listenPort, upstreamURL, authURL, redirectURL)
	if err != nil {
		return errors.Wrap(err, "Failed to create auth-proxy server")
	}
	if err := server.start(); err != nil {
		return errors.Wrap(err, "Failed to start auth-proxy server")
	}
	select {}
}

// newServer builds the auth-proxy for the given mode. The mode decides only where it listens and what it
// serves: reverse-proxy is exposed to the cluster on a configurable port and forwards to the processor;
// auth-only is reachable only from within the pod (loopback) and serves the DLX /auth endpoint.
func newServer(parentLogger logger.Logger,
	mode auth.ProxyMode,
	listenPort int,
	upstreamURL string,
	authURL string,
	redirectURL string) (*server, error) {

	var listenAddress string
	var handler http.Handler
	logger := parentLogger.GetChild("authproxy")

	switch mode {
	case auth.ProxyModeReverseProxy:
		listenAddress = fmt.Sprintf(":%d", listenPort)
		handler = newReverseProxyHandler(logger)
	case auth.ProxyModeAuthOnly:
		listenAddress = fmt.Sprintf("127.0.0.1:%d", listenPort)
		handler = newAuthOnlyHandler(logger)
	default:
		return nil, errors.Errorf("Unknown auth-proxy mode: %s", mode)
	}

	return &server{
		logger: logger,
		httpServer: &http.Server{
			Addr:    listenAddress,
			Handler: handler,
		},
		upstreamURL: upstreamURL,
		authURL:     authURL,
		redirectURL: redirectURL,
	}, nil
}

func (s *server) start() error {
	s.logger.InfoWith("Starting auth-proxy",
		"listenAddress", s.httpServer.Addr,
		"upstreamURL", s.upstreamURL,
		"authURL", s.authURL,
		"redirectURL", s.redirectURL)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.ErrorWith("Auth-proxy server stopped", "error", err)
		}
	}()

	return nil
}

// newReverseProxyHandler serves the running-function topology. It does not forward requests upstream yet:
// authenticating and proxying to the processor over loopback is implemented in NUC-828 / NUC-837.
func newReverseProxyHandler(logger logger.Logger) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		logger.Warn("reverse-proxy forwarding is not yet implemented")
		responseWriter.WriteHeader(http.StatusNotImplemented)
	})
}

// newAuthOnlyHandler serves the DLX /auth decision endpoint. Only /auth is routed; any other path
// returns 404, pinning the endpoint surface now so the NUC-837 decision logic slots into /auth.
func newAuthOnlyHandler(logger logger.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", func(responseWriter http.ResponseWriter, _ *http.Request) {
		//TODO - implement auth-only decision as part of NUC-837
		logger.Warn("auth-only mode was requested but is not yet implemented")
		responseWriter.WriteHeader(http.StatusNotImplemented)
	})
	return mux
}

func validateConfiguration(listenPort int, upstreamURL string, authURL string, redirectURL string) error {
	if listenPort == 0 {
		return errors.Errorf("Invalid listen port: %d", listenPort)
	}

	if upstreamURL == "" {
		return errors.New("Upstream URL must be provided")
	}

	if authURL == "" {
		return errors.New("Auth URL must be provided")
	}

	if redirectURL == "" {
		return errors.New("Redirect URL must be provided")
	}

	return nil
}
