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
	AllowOrigin      string
	AllowMethods     []string
	AllowHeaders     []string
	AllowCredentials bool

	// preflight
	PreflightRequestMethod string
	PreflightMaxAgeSeconds int64

	// computed
	allowMethodsStr           string
	allowHeadersStr           string
	preflightMaxAgeSecondsStr string
	allowCredentialsStr       string
	simpleMethods             []string
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

func (c *CORS) IsSetAndMatchOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	return c.AllowOrigin == "*" || origin == c.AllowOrigin
}

func (c *CORS) IsSetAndMatchMethod(method string) bool {
	return method != "" &&
		(method == c.PreflightRequestMethod || common.StringSliceContainsString(c.AllowMethods, method))
}

func (c *CORS) AreMatchHeaders(headers []string) bool {
	for _, header := range headers {
		if !common.StringSliceContainsStringCaseInsensitive(c.AllowHeaders, header) {
			return false
		}
	}
	return true
}

func (c *CORS) GetComputedAllowMethodsStr() string {
	if c.allowMethodsStr == "" {
		c.allowMethodsStr = strings.Join(c.AllowMethods, ", ")
	}
	return c.allowMethodsStr
}

func (c *CORS) GetComputedAllowHeadersStr() string {
	if c.allowHeadersStr == "" {
		c.allowHeadersStr = strings.Join(c.AllowHeaders, ", ")
	}
	return c.allowHeadersStr
}

func (c *CORS) GetComputedAllowCredentialsHeaderStr() string {
	if c.allowCredentialsStr == "" {
		c.allowCredentialsStr = strconv.FormatBool(c.AllowCredentials)
	}
	return c.allowHeadersStr
}

func (c *CORS) GetComputedPreflightMaxAgeSecondsStr() string {
	if c.preflightMaxAgeSecondsStr == "" {
		c.preflightMaxAgeSecondsStr = strconv.FormatInt(c.PreflightMaxAgeSeconds, 10)
	}
	return c.preflightMaxAgeSecondsStr
}
