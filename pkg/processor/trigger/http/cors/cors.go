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

package cors

import (
	"github.com/nuclio/errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/valyala/fasthttp"
)

type CORS struct {
	Enabled bool

	// allow configuration
	AllowOrigin      string
	AllowMethods     []string
	AllowHeaders     []string
	AllowCredentials bool

	// preflight
	PreflightRequestMethod string
	PreflightMaxAgeSeconds int

	// encoded
	allowMethodsStr           string
	allowHeadersStr           string
	preflightMaxAgeSecondsStr string
	allowCredentialsStr       string

	allowOriginURL *url.URL
}

func NewCORS() *CORS {
	return &CORS{
		Enabled:     true,
		AllowOrigin: "*",
		AllowMethods: []string{
			fasthttp.MethodHead,
			fasthttp.MethodGet,
			fasthttp.MethodPost,
			fasthttp.MethodPut,
			fasthttp.MethodDelete,
			fasthttp.MethodOptions,
		},
		AllowHeaders: []string{
			fasthttp.HeaderAccept,
			fasthttp.HeaderContentLength,
			fasthttp.HeaderContentType,

			// nuclio custom
			"X-nuclio-log-level",
		},
		AllowCredentials:       false,
		PreflightRequestMethod: fasthttp.MethodOptions,
		PreflightMaxAgeSeconds: -1, // disable cache by default
	}
}

func (c *CORS) OriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}

	// allow all
	if c.AllowOrigin == "*" {
		return true
	}

	// exact match
	if c.AllowOrigin == origin {
		return true
	}

	// at last, ensure host, port & schemes
	urlInstance, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return urlInstance.Host == c.allowOriginURL.Host &&
		urlInstance.Port() == c.allowOriginURL.Port() &&
		urlInstance.Scheme == c.allowOriginURL.Scheme

}

func (c *CORS) MethodAllowed(method string) bool {
	return method != "" &&
		(method == c.PreflightRequestMethod || common.StringSliceContainsStringCaseInsensitive(c.AllowMethods, method))
}

func (c *CORS) HeadersAllowed(headers []string) bool {
	for _, header := range headers {
		if !common.StringSliceContainsStringCaseInsensitive(c.AllowHeaders, header) {
			return false
		}
	}
	return true
}

func (c *CORS) EncodedAllowMethods() string {
	if c.allowMethodsStr == "" {
		c.allowMethodsStr = strings.Join(c.AllowMethods, ", ")
	}
	return c.allowMethodsStr
}

func (c *CORS) EncodeAllowHeaders() string {
	if c.allowHeadersStr == "" {
		c.allowHeadersStr = strings.Join(c.AllowHeaders, ", ")
	}
	return c.allowHeadersStr
}

func (c *CORS) EncodeAllowCredentialsHeader() string {
	if c.allowCredentialsStr == "" {
		c.allowCredentialsStr = strconv.FormatBool(c.AllowCredentials)
	}
	return c.allowCredentialsStr
}

func (c *CORS) EncodePreflightMaxAgeSeconds() string {
	if c.preflightMaxAgeSecondsStr == "" {
		c.preflightMaxAgeSecondsStr = strconv.Itoa(c.PreflightMaxAgeSeconds)
	}
	return c.preflightMaxAgeSecondsStr
}

func (c *CORS) SetAllowOriginURL(rawURL string) error {
	c.AllowOrigin = rawURL

	// skip parsing url when allow origin is broad
	if rawURL == "*" {
		return nil
	}

	urlInstance, err := url.Parse(rawURL)
	if err != nil {
		return errors.Wrap(err, "Failed to parse request URI")
	}
	c.allowOriginURL = urlInstance
	return nil
}
