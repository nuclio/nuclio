//go:build test_unit

/*
Copyright 2018 The Nuclio Authors.

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

package http

import (
	"context"
	"net"
	nethttp "net/http"
	"testing"

	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/cors"

	"github.com/stretchr/testify/suite"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

type TestSuite struct {
	processorsuite.TestSuite
	trigger http

	fastDummyHTTPServer        *fasthttputil.InmemoryListener
	fastDummyHTTPServerStarted bool
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()
	suite.trigger = http{
		AbstractTrigger: trigger.AbstractTrigger{
			Logger: suite.Logger,
		},
		configuration: &Configuration{},
	}
	suite.fastDummyHTTPServer = fasthttputil.NewInmemoryListener()
	suite.serveDummyHTTPServer(suite.trigger.onRequestFromFastHTTP())
}

func (suite *TestSuite) TearDownSuite() {
	suite.stopDummyHTTPServer()
}

func (suite *TestSuite) TestCORS() {
	client := suite.getClient()
	for _, testCase := range []struct {
		CORSAllowOrigins                  []string
		RequestOrigin                     string
		RequestMethod                     string
		RequestHeaders                    []string
		ExpectedResponseStatusCode        int
		ExpectedResponseHeaders           map[string]string
		ExpectedEventsHandledSuccessTotal uint64
		ExpectedEventsHandledFailureTotal uint64
	}{

		// happy flow
		{
			CORSAllowOrigins: []string{"foo.bar"},
			RequestOrigin:    "foo.bar",
			RequestMethod:    "GET",
			RequestHeaders: []string{
				"X-Nuclio-log-level",
			},
			ExpectedResponseStatusCode: fasthttp.StatusOK,
			ExpectedResponseHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "foo.bar",
				"Access-Control-Allow-Methods": "HEAD, GET, POST, PUT, DELETE, OPTIONS",
				"Access-Control-Max-Age":       "5",
				"Access-Control-Allow-Headers": "Accept, Content-Length, Content-Type, Authorization, X-nuclio-log-level",
			},
			ExpectedEventsHandledSuccessTotal: 1,
			ExpectedEventsHandledFailureTotal: 0,
		},

		// invalid origin
		{
			CORSAllowOrigins:                  []string{"foo.bar"},
			RequestOrigin:                     "baz.bar",
			RequestMethod:                     "GET",
			ExpectedResponseStatusCode:        fasthttp.StatusBadRequest,
			ExpectedEventsHandledSuccessTotal: 0,
			ExpectedEventsHandledFailureTotal: 1,
		},

		// invalid request header
		{
			RequestOrigin: "foo.bar",
			RequestMethod: "GET",
			RequestHeaders: []string{
				"Not-supported-header",
			},
			ExpectedResponseStatusCode:        fasthttp.StatusBadRequest,
			ExpectedEventsHandledSuccessTotal: 0,
			ExpectedEventsHandledFailureTotal: 1,
		},

		// invalid request method
		{
			RequestOrigin:                     "foo.bar",
			RequestMethod:                     "ABC",
			ExpectedResponseStatusCode:        fasthttp.StatusBadRequest,
			ExpectedEventsHandledSuccessTotal: 0,
			ExpectedEventsHandledFailureTotal: 1,
		},
	} {
		suite.Logger.DebugWith("Testing CORS", "testCase", testCase)

		// set cors configuration
		corsInstance := cors.NewCORS()
		if len(testCase.CORSAllowOrigins) > 0 {
			corsInstance.AllowOrigins = testCase.CORSAllowOrigins
		}
		suite.trigger.configuration.CORS = corsInstance

		// reset statistics
		suite.trigger.Statistics.EventsHandledSuccessTotal = 0
		suite.trigger.Statistics.EventsHandledFailureTotal = 0

		// ensure trigger is ready
		suite.trigger.status = status.Ready

		// create request, use OPTIONS to trigger preflight flow
		request, err := nethttp.NewRequest(fasthttp.MethodOptions, "http://foo.bar/", nil)
		suite.Require().NoError(err, "Failed to create new request")

		// set preflight required headers
		request.Header.Set("Origin", testCase.RequestOrigin)
		request.Header.Set("Access-Control-Request-Method", testCase.RequestMethod)
		for _, headerName := range testCase.RequestHeaders {
			request.Header.Add("Access-Control-Request-Headers", headerName)
		}

		// do request
		response, err := client.Do(request)
		suite.Require().NoError(err, "Failed to do request")
		suite.Logger.DebugWith("Received response",
			"headers", response.Header,
			"statusCode", response.StatusCode)

		// check for response status code
		suite.Equal(testCase.ExpectedResponseStatusCode, response.StatusCode)

		// check for response headers
		for headerName, headerValue := range testCase.ExpectedResponseHeaders {
			suite.Equal(response.Header.Get(headerName), headerValue)
		}

		// check statistic were update correspondingly
		suite.Equal(testCase.ExpectedEventsHandledSuccessTotal,
			suite.trigger.Statistics.EventsHandledSuccessTotal)

		// check statistic were update correspondingly
		suite.Equal(testCase.ExpectedEventsHandledFailureTotal,
			suite.trigger.Statistics.EventsHandledFailureTotal)

	}
}

func (suite *TestSuite) TestInternalHealthiness() {
	client := suite.getClient()
	for _, testCase := range []struct {
		name string
	}{
		{
			name: "sanity",
		},
	} {

		suite.Run(testCase.name, func() {
			suite.Logger.DebugWith("Testing internal healthiness endpoint", "testCase", testCase)

			// ensure trigger is ready
			suite.trigger.status = status.Ready

			request, err := nethttp.NewRequest(nethttp.MethodGet,
				"http://foo.bar"+string(suite.trigger.internalHealthPath),
				nil)
			suite.Require().NoError(err, "Failed to create new request")

			// do request
			response, err := client.Do(request)
			suite.Require().NoError(err, "Failed to do request")
			suite.Logger.DebugWith("Received response",
				"headers", response.Header,
				"statusCode", response.StatusCode)

			suite.Require().Equal(response.StatusCode, nethttp.StatusOK)
		})

	}
}

func (suite *TestSuite) serveDummyHTTPServer(handler fasthttp.RequestHandler) {
	go func() {
		suite.fastDummyHTTPServerStarted = true
		if err := fasthttp.Serve(suite.fastDummyHTTPServer, handler); err != nil {
			suite.Require().NoError(err, "Failed to serve")
		}
	}()
}

func (suite *TestSuite) stopDummyHTTPServer() {
	if suite.fastDummyHTTPServerStarted {
		suite.fastDummyHTTPServer.Close() // nolint: errcheck
	}
	suite.fastDummyHTTPServerStarted = false
}

func (suite *TestSuite) getClient() *nethttp.Client {
	return &nethttp.Client{
		Transport: &nethttp.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return suite.fastDummyHTTPServer.Dial()
			},
		},
	}
}

func TestHTTPSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
