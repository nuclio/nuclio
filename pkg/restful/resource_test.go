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

	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/go-chi/chi"
	"github.com/nuclio/nuclio-sdk"
	"github.com/stretchr/testify/suite"
)

//
// Foo resource
//

type fooResource struct {
	*AbstractResource
}

func (fr *fooResource) GetSingle(request *http.Request) (string, Attributes) {
	return "fooID", Attributes{
		"a1": "v1",
		"a2": 2,
	}
}

func (fr *fooResource) GetByID(request *http.Request, id string) Attributes {
	if id == "dont_find_me" {
		return nil
	}

	return Attributes{
		"got_id": id,
	}
}

func (fr *fooResource) GetCustomRoutes() map[string]CustomRoute {
	return map[string]CustomRoute{
		"/{id}/single": {http.MethodGet, fr.getCustomSingle},
		"/{id}/multi":  {http.MethodGet, fr.getCustomMulti},
		"/post":        {http.MethodPost, fr.postCustom},
	}
}

func (fr *fooResource) Create(request *http.Request) (string, Attributes, error) {
	return "123", Attributes{
		"a": "b",
	}, nil
}

func (fr *fooResource) Update(request *http.Request, id string) (Attributes, error) {
	return Attributes{
		"a": "b",
	}, nil
}

func (fr *fooResource) Remove(request *http.Request, id string) error {
	return nil
}

func (fr *fooResource) getCustomSingle(request *http.Request) (string, map[string]Attributes, bool, int, error) {
	resourceID := chi.URLParam(request, "id")

	return "getCustomSingle", map[string]Attributes{
		resourceID: {"a": "b", "c": "d"},
	}, true, http.StatusOK, nil
}

func (fr *fooResource) getCustomMulti(request *http.Request) (string, map[string]Attributes, bool, int, error) {
	resourceID := chi.URLParam(request, "id")

	return "getCustomMulti", map[string]Attributes{
		resourceID: {"a": "b", "c": "d"},
	}, false, http.StatusOK, nil
}

func (fr *fooResource) postCustom(request *http.Request) (string, map[string]Attributes, bool, int, error) {
	return "postCustom", nil, true, http.StatusConflict, nil
}

//
// Moo resource
//

type mooResource struct {
	*AbstractResource
}

func (mr *mooResource) GetAll(request *http.Request) map[string]Attributes {
	return map[string]Attributes{
		"123": {
			"a1": "v1",
			"a2": 2,
		},
	}
}

func (mr *mooResource) Create(request *http.Request) (string, Attributes, error) {
	return "", nil, nuclio.ErrConflict
}

func (mr *mooResource) Update(request *http.Request, id string) (Attributes, error) {
	return nil, nil
}

func (mr *mooResource) Remove(request *http.Request, id string) error {
	return nuclio.ErrNotFound
}

//
// Boo resource
//

type booResource struct {
	*AbstractResource
}

func (br *booResource) Update(request *http.Request, id string) (Attributes, error) {
	return nil, nuclio.ErrNotFound
}

//
// Test suite
//

type ResourceTestSuite struct {
	suite.Suite
	logger         nuclio.Logger
	fooResource    *fooResource
	mooResource    *mooResource
	booResource    *booResource
	router         chi.Router
	testHTTPServer *httptest.Server
}

func (suite *ResourceTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")

	// root router
	suite.router = chi.NewRouter()

	//
	// create the foo resource
	//

	suite.fooResource = &fooResource{
		AbstractResource: NewAbstractResource("foo", []ResourceMethod{
			ResourceMethodGetList,
			ResourceMethodGetDetail,
			ResourceMethodCreate,
			ResourceMethodUpdate,
			ResourceMethodDelete,
		}),
	}
	suite.fooResource.Resource = suite.fooResource

	suite.registerResource("foo", suite.fooResource.AbstractResource)

	//
	// create the moo resource
	//

	suite.mooResource = &mooResource{
		AbstractResource: NewAbstractResource("moo", []ResourceMethod{
			ResourceMethodGetList,
			ResourceMethodCreate,
			ResourceMethodUpdate,
			ResourceMethodDelete,
		}),
	}
	suite.mooResource.Resource = suite.mooResource

	suite.registerResource("moo", suite.mooResource.AbstractResource)

	//
	// create the boo resource
	//

	suite.booResource = &booResource{
		AbstractResource: NewAbstractResource("boo", []ResourceMethod{
			ResourceMethodUpdate,
		}),
	}
	suite.booResource.Resource = suite.booResource

	suite.registerResource("boo", suite.booResource.AbstractResource)

	// set the router as the handler for requests
	suite.testHTTPServer = httptest.NewServer(suite.router)
}

func (suite *ResourceTestSuite) TearDownTest() {
	suite.testHTTPServer.Close()
}

func (suite *ResourceTestSuite) TestResourceServer() {
	// suite.Require().Equal(suite.processor, suite.fooResource.processor)
}

func (suite *ResourceTestSuite) TestFooResourceGetList() {
	suite.sendRequest("GET", "/foo", nil, nil, `{
		"id": "fooID",
		"a1": "v1",
		"a2": 2
	}`)
}

func (suite *ResourceTestSuite) TestFooResourceGetDetail() {
	suite.sendRequest("GET", "/foo/300", nil, nil, `{
		"id": "300",
		"got_id": "300"
	}`)
}

func (suite *ResourceTestSuite) TestFooResourceGetDetailNotFound() {
	code := http.StatusNotFound
	suite.sendRequest("GET", "/foo/dont_find_me", nil, &code, ``)
}

func (suite *ResourceTestSuite) TestFooResourceGetCustomSingle() {
	suite.sendRequest("GET", "/foo/abc/single", nil, nil, `{
		"id": "abc",
		"a": "b",
		"c": "d"
	}`)
}

func (suite *ResourceTestSuite) TestFooResourceGetCustomMulti() {
	suite.sendRequest("GET", "/foo/abc/multi", nil, nil, `{
		"abc": {
			"a": "b",
			"c": "d"
		}
	}`)
}

func (suite *ResourceTestSuite) TestFooResourcePostCustom() {
	code := http.StatusConflict

	suite.sendRequest("POST", "/foo/post", nil, &code, `{}`)
}

func (suite *ResourceTestSuite) TestFooResourceCreate() {
	code := http.StatusCreated
	suite.sendRequest("POST", "/foo", nil, &code, `{
		"id": "123",
		"a": "b"
	}`)
}

func (suite *ResourceTestSuite) TestFooResourceUpdate() {
	code := http.StatusOK
	suite.sendRequest("PUT", "/foo/444", nil, &code, `{
		"id": "444",
		"a": "b"
	}`)
}

func (suite *ResourceTestSuite) TestFooResourceDelete() {
	code := http.StatusNoContent
	suite.sendRequest("DELETE", "/foo/123", nil, &code, "")
}

func (suite *ResourceTestSuite) TestMooResourceGetList() {
	suite.sendRequest("GET", "/moo", nil, nil, `{
		"123": {
			"a1": "v1",
			"a2": 2
		}
	}`)
}

func (suite *ResourceTestSuite) TestMooResourceCreate() {
	code := http.StatusConflict
	suite.sendRequest("POST", "/moo", nil, &code, "")
}

func (suite *ResourceTestSuite) TestMooResourceUpdate() {
	code := http.StatusNoContent
	suite.sendRequest("PUT", "/moo/444", nil, &code, "")
}

func (suite *ResourceTestSuite) TestMooResourceDelete() {
	code := http.StatusNotFound
	suite.sendRequest("DELETE", "/moo/123", nil, &code, "")
}

func (suite *ResourceTestSuite) TestBooResourceUpdate() {
	code := http.StatusNotFound
	suite.sendRequest("PUT", "/boo/444", nil, &code, "")
}

func (suite *ResourceTestSuite) registerResource(name string, resource *AbstractResource) {

	// initialize the resource
	resource.Initialize(suite.logger, nil)

	// mount it
	suite.router.Mount("/"+name, resource.router)
}

func (suite *ResourceTestSuite) sendRequest(method string,
	path string,
	requestBody io.Reader,
	expectedStatusCode *int,
	encodedExpectedResponseBody string) (*http.Response, map[string]interface{}) {

	request, err := http.NewRequest(method, suite.testHTTPServer.URL+path, nil)
	suite.Require().NoError(err)

	response, err := http.DefaultClient.Do(request)
	suite.Require().NoError(err)

	encodedResponseBody, err := ioutil.ReadAll(response.Body)
	suite.Require().NoError(err)

	defer response.Body.Close()

	suite.logger.DebugWith("Got response", "response", string(encodedResponseBody))

	// check if status code was passed
	if expectedStatusCode != nil {
		suite.Require().Equal(*expectedStatusCode, response.StatusCode)
	}

	// if there's an expected status code, verify it
	decodedResponseBody := map[string]interface{}{}

	// if we need to compare bodies
	if encodedExpectedResponseBody != "" {

		err = json.Unmarshal(encodedResponseBody, &decodedResponseBody)
		suite.Require().NoError(err)

		suite.logger.DebugWith("Comparing expected",
			"expected", suite.cleanJSONstring(encodedExpectedResponseBody))

		decodedExpectedResponseBody := map[string]interface{}{}

		err = json.Unmarshal([]byte(encodedExpectedResponseBody), &decodedExpectedResponseBody)
		suite.Require().NoError(err)

		suite.Require().True(reflect.DeepEqual(decodedExpectedResponseBody, decodedResponseBody))
	}

	return response, decodedResponseBody
}

// remove tabs and newlines
func (suite *ResourceTestSuite) cleanJSONstring(input string) string {
	for _, char := range []string{"\n", "\t"} {
		input = strings.Replace(input, char, "", -1)
	}

	return input
}

func TestResourceTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceTestSuite))
}
