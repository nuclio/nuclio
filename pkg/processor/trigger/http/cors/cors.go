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

	"github.com/valyala/fasthttp"
)

type CORS struct {
	Enabled bool

	// allow configuration
	AllowOrigins     []string
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

	// true when once of `AllowOrigins` equals to "*"
	allowAllOrigins *bool
}

func NewCORS() *CORS {
	return &CORS{
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

	// when all origins are allowed
	if c.resolveAllowAllOrigins() {
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

func (c *CORS) resolveAllowAllOrigins() bool {
	if c.allowAllOrigins == nil {
		false := false
		c.allowAllOrigins = &false
		for _, allowOrigin := range c.AllowOrigins {
			if allowOrigin == "*" {
				true := true
				c.allowAllOrigins = &true
				break
			}
		}
	}
	return *c.allowAllOrigins
}
