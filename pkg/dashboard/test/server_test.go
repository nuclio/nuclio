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

package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/dashboard"
	_ "github.com/nuclio/nuclio/pkg/dashboard/resource"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type DashboardTestSuite struct {
	suite.Suite
	logger logger.Logger
	dashboardServer *dashboard.Server
	httpServer *httptest.Server
}

func (suite *DashboardTestSuite) SetupTest() {
	var err error

	suite.logger, _ = nucliozap.NewNuclioZapTest("test")

	// create a mock platform
	suite.dashboardServer, err = dashboard.NewServer(suite.logger,
		"",
		"",
		"",
		"",
		nil,
		true,
		&platformconfig.WebServer{},
		nil)

	if err != nil {
		panic("Failed to create server")
	}

	// create an http server from the dashboard server
	suite.httpServer = httptest.NewServer(suite.dashboardServer.Router)
}

func (suite *DashboardTestSuite) TeardownTest() {
	suite.httpServer.Close()
}

func (suite *DashboardTestSuite) TestCreateSuccessful() {
	http.Get(suite.httpServer.URL + "/functions")
}

func TestInlineParserTestSuite(t *testing.T) {
	suite.Run(t, new(DashboardTestSuite))
}
