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
	"strconv"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"

	"github.com/valyala/fasthttp"
)

type CORS struct {
	Enabled bool

	// allow configuration
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	AllowCredentials bool
	ExposeHeaders    []string

	// preflight
	PreflightRequestMethod string
	PreflightMaxAgeSeconds *int

	// encoded
	allowMethodsStr           string
	allowHeadersStr           string
	preflightMaxAgeSecondsStr string
	allowCredentialsStr       string
	exposeHeadersStr          string
}

func NewCORS() *CORS {
	cors := &CORS{
		Enabled: true,
		AllowOrigins: []string{
			"*",
		},
		AllowMethods: []string{
			fasthttp.MethodHead,
			fasthttp.MethodGet,
			fasthttp.MethodPost,
			fasthttp.MethodPut,
			fasthttp.MethodDelete,
			fasthttp.MethodOptions,
			fasthttp.MethodPatch,
		},
		AllowHeaders: []string{
			fasthttp.HeaderAccept,
			fasthttp.HeaderContentLength,
			fasthttp.HeaderContentType,
			fasthttp.HeaderAuthorization,

			// nuclio custom
			headers.LogLevel,
		},
		ExposeHeaders:          []string{},
		AllowCredentials:       false,
		PreflightRequestMethod: fasthttp.MethodOptions,
	}
	cors.populatePreflightMaxAgeSeconds()
	return cors
}

func (c *CORS) OriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}

	// when all origins are allowed
	if c.isAllowAllOrigins() {
		return true
	}

	// check one-by-one
	return common.StringSliceContainsStringCaseInsensitive(c.AllowOrigins, origin)
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

func (c *CORS) EncodeExposeHeaders() string {
	if c.exposeHeadersStr == "" {
		c.exposeHeadersStr = strings.Join(c.ExposeHeaders, ", ")
	}
	return c.exposeHeadersStr
}

func (c *CORS) EncodeAllowCredentialsHeader() string {
	if c.allowCredentialsStr == "" {
		c.allowCredentialsStr = strconv.FormatBool(c.AllowCredentials)
	}
	return c.allowCredentialsStr
}

func (c *CORS) EncodePreflightMaxAgeSeconds() string {

	// defensive programing
	c.populatePreflightMaxAgeSeconds()

	if c.preflightMaxAgeSecondsStr == "" {
		c.preflightMaxAgeSecondsStr = strconv.Itoa(*c.PreflightMaxAgeSeconds)
	}
	return c.preflightMaxAgeSecondsStr
}

func (c *CORS) isAllowAllOrigins() bool {
	for _, allowOrigin := range c.AllowOrigins {
		if allowOrigin == "*" {
			return true
		}
	}
	return false
}

func (c *CORS) populatePreflightMaxAgeSeconds() {
	if c.PreflightMaxAgeSeconds == nil {
		five := 5
		c.PreflightMaxAgeSeconds = &five
	}
}
