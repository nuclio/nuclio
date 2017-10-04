package common

import (
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/jarcoal/httpmock.v1"
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

func (ts *DownloadFileTestSuite) TestDownloadFile() {
	content := "content"
	errResult := ts.testDownloadFile(func(req *http.Request) (*http.Response, error) {
		responder := httpmock.NewStringResponder(200, content)
		response, err := responder(req)
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

	return DownloadFile(url, os.DevNull)
}

func TestIsURLTestSuite(t *testing.T) {
	suite.Run(t, new(IsURLTestSuite))
}

func TestDownloadFileTestSuite(t *testing.T) {
	suite.Run(t, new(DownloadFileTestSuite))
}
