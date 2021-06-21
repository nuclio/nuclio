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
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nuclio/errors"
	"github.com/valyala/fasthttp"
)

const (
	HTTPPrefix      = "http://"
	HTTPSPrefix     = "https://"
	LocalFilePrefix = "file://"
)

func DownloadFile(url string, out *os.File, headers http.Header) error {
	client := http.Client{}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	request.Header = headers
	response, err := client.Do(request)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"Failed to download file. Received an unexpected status code: %d",
			response.StatusCode)
	}

	defer response.Body.Close() // nolint: errcheck

	written, err := io.Copy(out, response.Body)

	if err != nil {
		return err
	}

	if err := out.Close(); err != nil {
		return err
	}

	if response.ContentLength != -1 && written != response.ContentLength {
		return fmt.Errorf(
			"Downloaded file length (%d) is different than URL content length (%d)",
			written,
			response.ContentLength)
	}

	return nil
}

func IsURL(s string) bool {
	return strings.HasPrefix(s, HTTPPrefix) || strings.HasPrefix(s, HTTPSPrefix)
}

func IsLocalFileURL(s string) bool {
	return strings.HasPrefix(s, LocalFilePrefix)
}

// extracts absolute path to file from local file URL
// example: "file://path/to/file" -> "/path/to/file"
func GetPathFromLocalFileURL(s string) string {
	if IsLocalFileURL(s) {
		return "/" + strings.TrimPrefix(s, LocalFilePrefix)
	}
	return ""
}

// Normalizes URL Path
// examples:
// "" -> "/"
// "a" -> "/a/"
// "//a//b/c/" -> "/a/b/c/"
func NormalizeURLPath(p string) string {
	uri := fasthttp.URI{}
	uri.SetPath(p)
	res := uri.Path()

	// always finish with '/' in the end
	if res[len(res)-1] != '/' {
		res = append(res, '/')
	}

	return string(res)
}

// SendHTTPRequest Sends an http request
// ignore expectedStatusCode by setting it to 0
func SendHTTPRequest(method string,
	requestURL string,
	body []byte,
	headers map[string]string,
	cookies []*http.Cookie,
	expectedStatusCode int,
	insecure bool,
	timeout time.Duration) ([]byte, int, error) {

	// create client
	client := &http.Client{
		Timeout: timeout,
	}

	if insecure {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	// create request object
	req, err := http.NewRequest(method, requestURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, 0, errors.Wrap(err, "Failed to create http request")
	}

	// attach cookies
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	// attach headers
	for headerKey, headerValue := range headers {
		req.Header.Set(headerKey, headerValue)
	}

	// perform the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, errors.Wrap(err, "Failed to send HTTP request")
	}

	// read response body
	var responseBody []byte
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close() // nolint: errcheck

		responseBody, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, 0, errors.Wrap(err, "Failed to read response body")
		}
	}

	// validate status code is as expected
	if expectedStatusCode != 0 && resp.StatusCode != expectedStatusCode {
		return responseBody, resp.StatusCode, errors.Errorf(
			"Got unexpected response status code: %d. Expected: %d",
			resp.StatusCode,
			expectedStatusCode)
	}

	return responseBody, resp.StatusCode, nil
}
