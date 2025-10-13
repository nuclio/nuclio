/*
Copyright 2023 The Nuclio Authors.

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
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/valyala/fasthttp"
)

const (
	HTTPPrefix                      = "http://"
	HTTPSPrefix                     = "https://"
	LocalFilePrefix                 = "file://"
	urlPathRegex                    = `^([\/]?([a-zA-Z0-9\-_]+[\/]?)*|)$`
	explicitRegistryURLRegex        = `^[a-zA-Z0-9\.-]+(?::[0-9]+)?/.+:[a-zA-Z0-9\._-]+$`
	explicitOnbuildRegistryURLRegex = `^[a-zA-Z0-9\.\-:/]+/.+:[a-zA-Z0-9\._-]+-[a-zA-Z0-9\._-]+$`
)

var URLPathRegexpCompiled = regexp.MustCompile(urlPathRegex)

// ExplicitlyRegistryURLCompiled validates container image references with explicit tags.
// Pattern format: registry[:port]/path/to/image:tag
//
// Components (all are required except port):
//   - Registry domain: alphanumeric with dots/hyphens (e.g., docker.io, gcr.io, myregistry.com)
//   - Port: optional numeric value (e.g., :5000, :1234, :443)
//   - Image path: one or more path segments separated by slashes (e.g., python, library/ubuntu, org/team/app)
//   - Tag: required alphanumeric identifier with dots, underscores, or hyphens (e.g., latest, 3.12, v1.0.0-alpine)
//
// Valid examples:
//   - function.registry:1234/python:3.12
//   - docker.io/library/ubuntu:22.04
//   - gcr.io/my-project/some/path/app:latest
var ExplicitlyRegistryURLCompiled = regexp.MustCompile(explicitRegistryURLRegex)

// ExplicitlyOnbuildRegistryURLCompiled validates onbuild handler builder images.
// Pattern format: registry[:port]/path/to/image:tag-arch
//
// Components:
//   - Registry: domain with optional port (e.g., quay.io, some.registry.com)
//   - Port: optional numeric value (e.g., :5000, :1234, :443)
//   - Image path: any valid path (e.g., nuclio/handler-builder-python, custom/path/to/builder)
//   - Tag:  any non-empty alphanumeric tag with dots, underscores, or hyphens (e.g., 3.12, v2.0, stable)
//   - Arch: any non-empty alphanumeric tag with dots, underscores, or hyphens (e.g., amd64, alpine)
//
// Valid examples:
//   - quay.io/nuclio/handler-builder-python-onbuild:1.13.0-amd64
//   - quay.io/nuclio/handler-builder-python-onbuild:latest-arm64
//   - registry.com:5000/custom/path/builder-onbuild:v2.0-alpine
var ExplicitlyOnbuildRegistryURLCompiled = regexp.MustCompile(explicitOnbuildRegistryURLRegex)

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
		return errors.Errorf(
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
		return errors.Errorf(
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

// GetPathFromLocalFileURL extracts absolute path to file from local file URL
// example: "file://path/to/file" -> "/path/to/file"
func GetPathFromLocalFileURL(s string) string {
	if IsLocalFileURL(s) {
		return "/" + strings.TrimPrefix(s, LocalFilePrefix)
	}
	return ""
}

// NormalizeURLPath normalizes URL Path
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

// ValidateURLPath validates only path of url (without host)
func ValidateURLPath(path string) bool {
	return URLPathRegexpCompiled.MatchString(path)
}

// SendHTTPRequest Sends an HTTP request using custom http client
// ignore expectedStatusCode by setting it to 0
func SendHTTPRequest(httpClient *http.Client,
	method string,
	requestURL string,
	body []byte,
	headers map[string]string,
	cookies []*http.Cookie,
	expectedStatusCode int) ([]byte, *http.Response, error) {
	return sendHTTPRequest(context.Background(),
		httpClient,
		method,
		requestURL,
		body,
		headers,
		cookies,
		expectedStatusCode)
}

// SendHTTPRequestWithContext is like SendHTTPRequest but with context
func SendHTTPRequestWithContext(ctx context.Context,
	httpClient *http.Client,
	method string,
	requestURL string,
	body []byte,
	headers map[string]string,
	cookies []*http.Cookie,
	expectedStatusCode int) ([]byte, *http.Response, error) {
	return sendHTTPRequest(ctx,
		httpClient,
		method,
		requestURL,
		body,
		headers,
		cookies,
		expectedStatusCode)
}

func sendHTTPRequest(ctx context.Context,
	httpClient *http.Client,
	method string,
	requestURL string,
	body []byte,
	headers map[string]string,
	cookies []*http.Cookie,
	expectedStatusCode int) ([]byte, *http.Response, error) {

	// create request object
	req, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to create http request")
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
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to send HTTP request")
	}

	// read response body
	var responseBody []byte
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close() // nolint: errcheck

		responseBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, errors.Wrap(err, "Failed to read response body")
		}
	}

	// validate status code is as expected
	if expectedStatusCode != 0 && resp.StatusCode != expectedStatusCode {
		return responseBody, resp, nuclio.GetByStatusCode(resp.StatusCode)(fmt.Sprintf(
			"Got unexpected response status code: %d. Expected: %d",
			resp.StatusCode,
			expectedStatusCode))
	}

	return responseBody, resp, nil
}

// CookiesToHeaderValue transforms a slice of cookies into a single header value string
func CookiesToHeaderValue(cookies []*http.Cookie) string {
	if len(cookies) == 0 {
		return ""
	}

	cookieStrings := make([]string, 0)
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		cookieStrings = append(cookieStrings, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}
	return strings.Join(cookieStrings, "; ")
}

// IsExplicitRegistryURL checks if the registry string is an explicit registry URL
// that includes a path component after the host[:port].
func IsExplicitRegistryURL(registry string) bool {
	if registry == "" {
		return false
	}

	return ExplicitlyRegistryURLCompiled.MatchString(registry)
}


// IsExplicitOnbuildRegistryURL checks if the onbuild registry string is an explicit registry URL
func IsExplicitOnbuildRegistryURL(registry string) bool {
	if registry == "" {
		return false
	}

	return ExplicitlyOnbuildRegistryURLCompiled.MatchString(registry)
}