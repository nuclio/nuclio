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

package restful

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

//
// Base resource
//
type resource struct {
	*AbstractResource
}

func (r *resource) respondWithError(request *http.Request) (bool, error) {
	if request.Header.Get("return") == "nil" {
		return true, nil
	}

	if request.Header.Get("return") == "error-golang" {
		return true, errors.New("GOLANG")
	}

	if request.Header.Get("return") == "error-with-status-202" {
		return true, nuclio.ErrAccepted
	}

	if request.Header.Get("return") == "error-with-status-409" {
		return true, nuclio.ErrConflict
	}

	if request.Header.Get("return") == "error-with-status-400" {
		return true, nuclio.NewErrBadRequest("BADREQUEST")
	}

	if request.Header.Get("return") == "error-with-status-400-wrapped" {
		return true, nuclio.WrapErrBadRequest(errors.Wrap(errors.New("ORIGINAL_ERROR"), "BADREQUEST_WRAPPED"))
	}

	// don't respond with error
	return false, nil
}

//
// Test suite
//

type resourceTestSuite struct {
	suite.Suite
	logger         logger.Logger
	router         chi.Router
	testHTTPServer *httptest.Server
}

func (suite *resourceTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")

	// root router
	suite.router = chi.NewRouter()

	// set the router as the handler for requests
	suite.testHTTPServer = httptest.NewServer(suite.router)
}

func (suite *resourceTestSuite) TearDownTest() {
	suite.testHTTPServer.Close()
}

func (suite *resourceTestSuite) registerResource(name string, resource *AbstractResource) {
	var err error

	// initialize the resource
	_, err = resource.Initialize(suite.logger, nil)
	suite.Require().NoError(err)

	// mount it
	suite.router.Mount("/"+name, resource.router)
}

func (suite *resourceTestSuite) sendRequest(method string,
	path string,
	requestHeaders map[string]string,
	requestBody io.Reader,
	expectedStatusCode *int,
	encodedExpectedResponse interface{},
	expectedResponseHeaders map[string][]string) (*http.Response, map[string]interface{}) {

	request, err := http.NewRequest(method, suite.testHTTPServer.URL+path, nil)
	suite.Require().NoError(err)

	for headerKey, headerValue := range requestHeaders {
		request.Header.Set(headerKey, headerValue)
	}

	response, err := http.DefaultClient.Do(request)
	suite.Require().NoError(err)

	encodedResponseBody, err := ioutil.ReadAll(response.Body)
	suite.Require().NoError(err)

	defer response.Body.Close()

	suite.logger.DebugWith("Got response", "status", response.StatusCode, "response", string(encodedResponseBody))

	// check if status code was passed
	if expectedStatusCode != nil {
		suite.Require().Equal(*expectedStatusCode, response.StatusCode)
	}

	for headerName, headerValues := range expectedResponseHeaders {
		suite.Require().Equal(response.Header[headerName], headerValues, "header is missing from response")
	}

	// if there's an expected status code, Verify it
	decodedResponseBody := map[string]interface{}{}

	if encodedExpectedResponse != nil {

		err = json.Unmarshal(encodedResponseBody, &decodedResponseBody)
		suite.Require().NoError(err)

		suite.logger.DebugWith("Comparing expected", "expected", encodedExpectedResponse)

		switch typedEncodedExpectedResponse := encodedExpectedResponse.(type) {
		case string:
			decodedExpectedResponseBody := map[string]interface{}{}

			err = json.Unmarshal([]byte(typedEncodedExpectedResponse), &decodedExpectedResponseBody)
			suite.Require().NoError(err)

			suite.Require().True(reflect.DeepEqual(decodedExpectedResponseBody, decodedResponseBody))

		case func(response map[string]interface{}) bool:
			suite.Require().True(typedEncodedExpectedResponse(decodedResponseBody))
		}
	}

	return response, decodedResponseBody
}

// remove tabs and newlines
func (suite *resourceTestSuite) cleanJSONstring(input string) string {
	for _, char := range []string{"\n", "\t"} {
		input = strings.ReplaceAll(input, char, "")
	}

	return input
}

// send error triggering requests
func (suite *resourceTestSuite) sendErrorRequests(method string, path string) {
	var code int
	var headers map[string]string
	var ecv *ErrorContainsVerifier

	// handler returns an ErrorWithStatusCode, but status code is below 400
	code = http.StatusAccepted
	headers = map[string]string{"return": "error-with-status-202"}
	suite.sendRequest(method, path, headers, nil, &code, nil, nil)

	// handler returns errors.New() error
	code = http.StatusInternalServerError
	headers = map[string]string{"return": "error-golang"}
	ecv = NewErrorContainsVerifier(suite.logger, []string{"GOLANG"})
	suite.sendRequest(method, path, headers, nil, &code, ecv, nil)

	// handler returns an ErrorWithStatusCode, and status code is above 400
	code = http.StatusBadRequest
	ecv = NewErrorContainsVerifier(suite.logger, []string{"BADREQUEST"})
	headers = map[string]string{"return": "error-with-status-400"}
	suite.sendRequest(method, path, headers, nil, &code, ecv.Verify, nil)

	// handler returns a *wrapped* ErrorWithStatusCode, and status code is above 400
	code = http.StatusBadRequest
	ecv = NewErrorContainsVerifier(suite.logger, []string{"ORIGINAL_ERROR"})
	headers = map[string]string{"return": "error-with-status-400-wrapped"}
	suite.sendRequest(method, path, headers, nil, &code, ecv.Verify, nil)
}

//
// R1
//

// resource
type r1Resource struct {
	resource
}

func (r1 *r1Resource) GetAll(request *http.Request) (map[string]Attributes, error) {
	if respondWithError, err := r1.respondWithError(request); respondWithError {
		return nil, err
	}

	return map[string]Attributes{
		"r1ID": {
			"a1": "v1",
			"a2": 2,
		},
	}, nil
}

func (r1 *r1Resource) GetByID(request *http.Request, id string) (Attributes, error) {
	if respondWithError, err := r1.respondWithError(request); respondWithError {
		return nil, err
	}

	return Attributes{
		"got_id": id,
	}, nil
}

func (r1 *r1Resource) GetCustomRoutes() ([]CustomRoute, error) {
	return []CustomRoute{
		{Stream: false, Pattern: "/{id}/single", Method: http.MethodGet, RouteFunc: r1.getCustomSingle},
		{Stream: false, Pattern: "/{id}/multi", Method: http.MethodGet, RouteFunc: r1.getCustomMulti},
		{Stream: false, Pattern: "/post", Method: http.MethodPost, RouteFunc: r1.postCustom},
	}, nil
}

func (r1 *r1Resource) Create(request *http.Request) (string, Attributes, error) {
	if respondWithError, err := r1.respondWithError(request); respondWithError {
		return "", nil, err
	}

	return "123", Attributes{
		"a": "b",
	}, nil
}

func (r1 *r1Resource) Update(request *http.Request, id string) (Attributes, error) {
	if respondWithError, err := r1.respondWithError(request); respondWithError {
		return nil, err
	}

	return Attributes{
		"a": "b",
	}, nil
}

func (r1 *r1Resource) Delete(request *http.Request, id string) error {
	if respondWithError, err := r1.respondWithError(request); respondWithError {
		return err
	}

	return nil
}

func (r1 *r1Resource) getCustomSingle(request *http.Request) (*CustomRouteFuncResponse, error) {
	resourceID := chi.URLParam(request, "id")

	return &CustomRouteFuncResponse{
		ResourceType: "getCustomSingle",
		Resources: map[string]Attributes{
			resourceID: {"a": "b", "c": "d"},
		},
		Single:     true,
		StatusCode: http.StatusOK,
	}, nil
}

func (r1 *r1Resource) getCustomMulti(request *http.Request) (*CustomRouteFuncResponse, error) {
	resourceID := chi.URLParam(request, "id")

	return &CustomRouteFuncResponse{
		ResourceType: "getCustomMulti",
		Resources: map[string]Attributes{
			resourceID: {"a": "b", "c": "d"},
		},
		StatusCode: http.StatusOK,
	}, nil
}

func (r1 *r1Resource) postCustom(request *http.Request) (*CustomRouteFuncResponse, error) {

	return &CustomRouteFuncResponse{
		ResourceType: "postCustom",
		Headers: map[string]string{
			"h1": "h1v",
			"h2": "h2v",
		},
		Single:     true,
		StatusCode: http.StatusConflict,
	}, nil
}

// test suite
type r1TestSuite struct {
	resourceTestSuite
	r1Resource *r1Resource
}

func (suite *r1TestSuite) SetupTest() {
	suite.resourceTestSuite.SetupTest()

	suite.r1Resource = &r1Resource{
		resource: resource{
			AbstractResource: NewAbstractResource("r1", []ResourceMethod{
				ResourceMethodGetList,
				ResourceMethodGetDetail,
				ResourceMethodCreate,
				ResourceMethodUpdate,
				ResourceMethodDelete,
			}),
		},
	}

	suite.r1Resource.Resource = suite.r1Resource

	suite.registerResource("r1", suite.r1Resource.AbstractResource)
}

func (suite *r1TestSuite) TestGetList() {
	suite.sendRequest("GET", "/r1", nil, nil, nil, `{
		"r1ID": {
			"a1": "v1",
			"a2": 2
		}
	}`, map[string][]string{
		"Content-Type": {"application/json"},
	})
}

func (suite *r1TestSuite) TestGetListErrors() {

	// handler returns nil attributes and nil error - expect 200 with {} body
	code := http.StatusOK
	headers := map[string]string{"return": "nil"}
	suite.sendRequest("GET", "/r1", headers, nil, &code, `{}`, nil)

	suite.sendErrorRequests("GET", "/r1")
}

func (suite *r1TestSuite) TestGetDetail() {
	suite.sendRequest("GET", "/r1/300", nil, nil, nil, `{
		"got_id": "300"
	}`, map[string][]string{
		"Content-Type": {"application/json"},
	})
}

func (suite *r1TestSuite) TestGetDetailErrors() {

	// handler returns nil attributes and nil error - expect 400 with no body
	code := http.StatusNotFound
	headers := map[string]string{"return": "nil"}
	suite.sendRequest("GET", "/r1/300", headers, nil, &code, nil, nil)

	suite.sendErrorRequests("GET", "/r1/300")
}

func (suite *r1TestSuite) TestGetCustomSingle() {
	suite.sendRequest("GET", "/r1/abc/single", nil, nil, nil, `{
		"a": "b",
		"c": "d"
	}`, nil)
}

func (suite *r1TestSuite) TestGetCustomMulti() {
	suite.sendRequest("GET", "/r1/abc/multi", nil, nil, nil, `{
		"abc": {
			"a": "b",
			"c": "d"
		}
	}`, nil)
}

func (suite *r1TestSuite) TestPostCustom() {
	code := http.StatusConflict

	response, _ := suite.sendRequest("POST",
		"/r1/post",
		nil,
		nil,
		&code,
		nil,
		nil)

	suite.Require().Equal("h1v", response.Header.Get("h1"))
	suite.Require().Equal("h2v", response.Header.Get("h2"))
}

func (suite *r1TestSuite) TestCreate() {
	code := http.StatusCreated
	suite.sendRequest("POST", "/r1", nil, nil, &code, `{
		"a": "b"
	}`, map[string][]string{
		"Content-Type": {"application/json"},
	})
}

func (suite *r1TestSuite) TestCreateErrors() {
	suite.sendErrorRequests("POST", "/r1")
}

func (suite *r1TestSuite) TestUpdate() {
	code := http.StatusOK
	suite.sendRequest("PUT", "/r1/444", nil, nil, &code, `{
		"a": "b"
	}`, nil)
}

func (suite *r1TestSuite) TestUpdateErrors() {
	suite.sendErrorRequests("PUT", "/r1/444")
}

func (suite *r1TestSuite) TestDelete() {
	code := http.StatusNoContent
	suite.sendRequest("DELETE", "/r1/123", nil, nil, &code, nil, nil)
}

func (suite *r1TestSuite) TestDeleteErrors() {
	suite.sendErrorRequests("DELETE", "/r1/123")
}

//
// R2
//

// resource
type r2Resource struct {
	*AbstractResource
}

func (r2 *r2Resource) GetAll(request *http.Request) (map[string]Attributes, error) {
	return map[string]Attributes{
		"123": {
			"a1": "v1",
			"a2": 2,
		},
	}, nil
}

func (r2 *r2Resource) Create(request *http.Request) (string, Attributes, error) {
	return "", nil, nuclio.ErrConflict
}

func (r2 *r2Resource) Update(request *http.Request, id string) (Attributes, error) {
	return nil, nil
}

func (r2 *r2Resource) Delete(request *http.Request, id string) error {
	return nuclio.ErrNotFound
}

// test suite
type r2TestSuite struct {
	resourceTestSuite
	r2Resource *r2Resource
}

func (suite *r2TestSuite) SetupTest() {
	suite.resourceTestSuite.SetupTest()

	suite.r2Resource = &r2Resource{
		AbstractResource: NewAbstractResource("r2", []ResourceMethod{
			ResourceMethodGetList,
			ResourceMethodCreate,
			ResourceMethodUpdate,
			ResourceMethodDelete,
		}),
	}
	suite.r2Resource.Resource = suite.r2Resource

	suite.registerResource("r2", suite.r2Resource.AbstractResource)
}

func (suite *r2TestSuite) TestGetList() {
	suite.sendRequest("GET", "/r2", nil, nil, nil, `{
		"123": {
			"a1": "v1",
			"a2": 2
		}
	}`, map[string][]string{
		"Content-Type": {"application/json"},
	})
}

func (suite *r2TestSuite) TestCreate() {
	code := http.StatusConflict
	suite.sendRequest("POST", "/r2", nil, nil, &code, nil, map[string][]string{
		"Content-Type": {"application/json"},
	})
}

func (suite *r2TestSuite) TestUpdate() {
	code := http.StatusNoContent
	suite.sendRequest("PUT", "/r2/444", nil, nil, &code, nil, nil)
}

func (suite *r2TestSuite) TestDelete() {
	code := http.StatusNotFound
	suite.sendRequest("DELETE", "/r2/123", nil, nil, &code, nil, nil)
}

//
// R3
//

// resource
type r3Resource struct {
	*AbstractResource
}

func (r3 *r3Resource) Update(request *http.Request, id string) (Attributes, error) {
	return nil, nuclio.ErrNotFound
}

// test suite
type r3TestSuite struct {
	resourceTestSuite
	r3Resource *r3Resource
}

func (suite *r3TestSuite) SetupTest() {
	suite.resourceTestSuite.SetupTest()

	suite.r3Resource = &r3Resource{
		AbstractResource: NewAbstractResource("r3", []ResourceMethod{
			ResourceMethodUpdate,
		}),
	}

	suite.r3Resource.Resource = suite.r3Resource

	suite.registerResource("r3", suite.r3Resource.AbstractResource)
}

func (suite *r3TestSuite) TestUpdate() {
	code := http.StatusNotFound
	suite.sendRequest("PUT", "/r3/444", nil, nil, &code, nil, map[string][]string{
		"Content-Type": {"application/json"},
	})
}

//
// Run suites
//
func TestResourceTestSuite(t *testing.T) {
	suite.Run(t, new(r1TestSuite))
	suite.Run(t, new(r2TestSuite))
	suite.Run(t, new(r3TestSuite))
}
