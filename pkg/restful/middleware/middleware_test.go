//go:build test_unit

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

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type middlewareTestSuite struct {
	suite.Suite
	logger         logger.Logger
	router         chi.Router
	testHTTPServer *httptest.Server
}

func (suite *middlewareTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")

	// root router
	suite.router = chi.NewRouter()

	// set the router as the handler for requests
	suite.testHTTPServer = httptest.NewServer(suite.router)
}

func (suite *middlewareTestSuite) TearDownTest() {
	suite.testHTTPServer.Close()
}

func (suite *middlewareTestSuite) TestModifyIguazioRequestHeaderPrefix() {

	// create a handler to use as "next" which will verify the request headers
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suite.Require().NotContains(r.Header, "x-igz-nuclio-header")
		suite.Require().Contains(r.Header, "x-nuclio-header")

		// verify no `igz` left in headers
		for header := range r.Header {
			suite.Require().NotContains(header, "igz")

			// for case-sensitive headers
			suite.Require().NotContains(header, "Igz")
		}
	})

	// create the handler to test, using our custom "next" handler
	handlerToTest := ModifyIguazioRequestHeaderPrefix(nextHandler)

	// create a mock request to use
	req := httptest.NewRequest("GET", "http://some-url", nil)

	// add headers to request
	req.Header.Add("x-igz-nuclio-header", "some-value")
	req.Header.Add("regular-nuclio-header", "regular-value")

	// call the handler using a mock response recorder (we'll not use that anyway)
	handlerToTest.ServeHTTP(httptest.NewRecorder(), req)
}

func TestMiddlewareTestSuite(t *testing.T) {
	suite.Run(t, new(middlewareTestSuite))
}
