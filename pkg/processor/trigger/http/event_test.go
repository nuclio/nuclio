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
	"github.com/valyala/fasthttp"
	"testing"

	"github.com/stretchr/testify/suite"
)

type EventTestSuite struct {
	suite.Suite
	event Event
}

func (suite *EventTestSuite) SetupTest() {
	suite.event = Event{}
}

func (suite *EventTestSuite) TestGetEventPathDisablePathNormalization() {
	ctx := &fasthttp.RequestCtx{
		Request: fasthttp.Request{},
	}
	requestPath := "/../../am%20here"
	ctx.Request.URI().SetPath(requestPath)
	suite.event.ctx = ctx
	suite.event.disablePathNormalizing = true
	suite.Require().Equal(requestPath, suite.event.GetPath())
}

func (suite *EventTestSuite) TestGetEventPath() {
	ctx := &fasthttp.RequestCtx{
		Request: fasthttp.Request{},
	}
	requestPath := "/../../am%20here"
	ctx.Request.URI().SetPath(requestPath)
	suite.event.ctx = ctx
	suite.event.disablePathNormalizing = false
	suite.Require().Equal("/am here", suite.event.GetPath())
}

func TestEvent(t *testing.T) {
	suite.Run(t, new(EventTestSuite))
}
