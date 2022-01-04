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

package common

import (
	"net/http"
	"os"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/nuclio/errors"
	"github.com/stretchr/testify/suite"
)

type IsURLTestSuite struct {
	suite.Suite
}

type DownloadFileTestSuite struct {
	suite.Suite
}

func (ts *IsURLTestSuite) TestIsURLWithFile() {
	ts.Require().False(IsURL("/not/a/url"))
}

func (ts *IsURLTestSuite) TestIsURLWithFTP() {
	ts.Require().False(IsURL("ftp://www.example.com"))
}

func (ts *IsURLTestSuite) TestIsURLWithHTTP() {
	ts.Require().True(IsURL("http://www.example.com"))
}

func (ts *IsURLTestSuite) TestIsURLWithHTTPS() {
	ts.Require().True(IsURL("https://www.example.com"))
}

func (ts *IsURLTestSuite) TestIsLocalFileURLWithHTTP() {
	ts.Require().False(IsLocalFileURL("http://www.example.com"))
}

func (ts *IsURLTestSuite) TestIsLocalFileURL() {
	ts.Require().True(IsLocalFileURL("file://path/to/file"))
}

func (ts *IsURLTestSuite) TestGetPathFromLocalFileURL() {
	ts.Require().Equal("/path/to/file", GetPathFromLocalFileURL("file://path/to/file"))
}

func (ts *IsURLTestSuite) TestNormalizeURLPath() {
	for inputPath, expectedOutputPath := range map[string]string{
		"":                     "/",
		"/":                    "/",
		"//":                   "/",
		"/////":                "/",
		"a":                    "/a/",
		"/a":                   "/a/",
		"/a/b":                 "/a/b/",
		"a//b////c":            "/a/b/c/",
		"/////a////bb/////ccc": "/a/bb/ccc/",
		"a/b/c/////":           "/a/b/c/",
	} {
		ts.Assert().Equal(expectedOutputPath, NormalizeURLPath(inputPath))
	}
}

func (ts *DownloadFileTestSuite) TestDownloadFile() {
	content := "content"
	errResult := ts.testDownloadFile(func(req *http.Request) (*http.Response, error) {
		responder := httpmock.NewStringResponder(200, content)
		response, err := responder(req)
		if err != nil {
			return nil, errors.Wrap(err, "Could not get response")
		}
		response.ContentLength = int64(len(content))
		return response, err
	})
	ts.Require().NoError(errResult, "Unable to download file")
}

func (ts *DownloadFileTestSuite) TestDownloadFileContentLengthMissMatch() {
	content := "content"
	errResult := ts.testDownloadFile(func(req *http.Request) (*http.Response, error) {
		responder := httpmock.NewStringResponder(200, content)
		response, err := responder(req)
		if err != nil {
			return nil, errors.Wrap(err, "Could not get response")
		}
		response.ContentLength = int64(len(content)) - 1
		return response, err
	})
	ts.Require().Error(errResult, "Should fail to download")
}

func (ts *DownloadFileTestSuite) TestDownloadURLError() {
	errResult := ts.testDownloadFile(httpmock.NewErrorResponder(errors.New("Something")))
	ts.Require().Error(errResult, "Should fail to download")
}

func (ts *DownloadFileTestSuite) testDownloadFile(responder httpmock.Responder) error {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	url := "http://www.example.com/file.txt"
	httpmock.RegisterResponder("GET", url, responder)

	out, err := os.Create(os.DevNull)
	if err != nil {
		return err
	}

	return DownloadFile(url, out, http.Header{})
}

func TestIsURLTestSuite(t *testing.T) {
	suite.Run(t, new(IsURLTestSuite))
}

func TestDownloadFileTestSuite(t *testing.T) {
	suite.Run(t, new(DownloadFileTestSuite))
}
