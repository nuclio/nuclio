package cors

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	cors *CORS
}

func (suite *TestSuite) SetupSuite() {
	suite.cors = NewCORS()
	suite.cors.Enabled = true
}

func (suite *TestSuite) TestEncodeExposeHeaders() {
	// empty by default
	suite.Require().Equal(suite.cors.EncodeExposeHeaders(), "")

	// empty lazy-load encoded string
	suite.cors.exposeHeadersStr = ""
	suite.cors.ExposeHeaders = []string{"x-nuclio-something", "x-nuclio-somethingelse"}
	suite.Require().Equal(suite.cors.EncodeExposeHeaders(), "x-nuclio-something, x-nuclio-somethingelse")
}

func (suite *TestSuite) TestEncodeAllowCredentialsHeader() {

	// false by default
	suite.Require().Equal(suite.cors.EncodeAllowCredentialsHeader(), "false")

	// empty lazy-load encoded string
	suite.cors.allowCredentialsStr = ""
	suite.cors.AllowCredentials = true
	suite.Require().Equal(suite.cors.EncodeAllowCredentialsHeader(), "true")
}

func (suite *TestSuite) TestEncodePreflightMaxAgeSeconds() {

	// 5 seconds by default
	suite.Require().Equal(suite.cors.EncodePreflightMaxAgeSeconds(), "5")

	// empty lazy-load encoded string
	zero := 0
	suite.cors.preflightMaxAgeSecondsStr = ""
	suite.cors.PreflightMaxAgeSeconds = &zero
	suite.Require().Equal(suite.cors.EncodePreflightMaxAgeSeconds(), "0")
}

func (suite *TestSuite) TestOriginAllowed() {
	for _, testCase := range []struct {
		allowOrigins []string
		origin       string
		valid        bool
	}{

		// happy flow, allow all
		{allowOrigins: []string{"*"}, origin: "a", valid: true},
		{allowOrigins: []string{"*"}, origin: "b", valid: true},
		{allowOrigins: []string{"*"}, origin: "c", valid: true},

		// allow specific only
		{allowOrigins: []string{"a"}, origin: "a", valid: true},
		{allowOrigins: []string{"a"}, origin: "A", valid: true},
		{allowOrigins: []string{"a"}, origin: "b", valid: false},
		{allowOrigins: []string{"a"}, origin: "B", valid: false},
		{allowOrigins: []string{"a"}, origin: "c", valid: false},
		{allowOrigins: []string{"a"}, origin: "C", valid: false},

		// allow both "a" & "b", deny for "c"
		{allowOrigins: []string{"a", "b"}, origin: "a", valid: true},
		{allowOrigins: []string{"a", "b"}, origin: "b", valid: true},
		{allowOrigins: []string{"a", "b"}, origin: "c", valid: false},

		// allow case insensitive
		{allowOrigins: []string{"a"}, origin: "A", valid: true},

		// exact match (not contains in)
		{allowOrigins: []string{"aa"}, origin: "a", valid: false},
		{allowOrigins: []string{"a"}, origin: "aa", valid: false},

		// real life, allow origin for "http" based origin / no scheme
		{
			allowOrigins: []string{
				"nuclio.io",
				"http://nuclio.io",
				"http://nuclio.io:80",
			},
			origin: "http://nuclio.io:80",
			valid:  true,
		},
		{
			allowOrigins: []string{
				"nuclio.io",
				"http://nuclio.io",
				"http://nuclio.io:80",
			},
			origin: "https://Nuclio.io",
			valid:  false,
		},

		// regardless to allow origin, empty host is against CORS RFC and should not be treated
		{allowOrigins: []string{"*"}, origin: "", valid: false},
	} {
		cors := NewCORS()
		if len(testCase.allowOrigins) > 0 {
			cors.AllowOrigins = testCase.allowOrigins
		}
		suite.Require().Equal(testCase.valid, cors.OriginAllowed(testCase.origin))
	}
}

func (suite *TestSuite) TestMethodsAllowed() {

	// regardless to allow origin, empty method is against CORS RFC and should not be treated
	suite.Require().False(suite.cors.MethodAllowed(""))

	// always allow preflight method (e.g.: OPTIONS)
	suite.Require().True(suite.cors.MethodAllowed(suite.cors.PreflightRequestMethod))

	for _, method := range suite.cors.AllowMethods {
		suite.Require().True(suite.cors.MethodAllowed(method))
	}

	// not enabled by default
	suite.Require().False(suite.cors.MethodAllowed(http.MethodTrace))
}

func (suite *TestSuite) TestHeadersAllowed() {
	dummyHeader := "Dummy-Header"

	// allow default headers
	suite.Require().True(suite.cors.HeadersAllowed(suite.cors.AllowHeaders))

	// dummyHeader should be denied at this point
	suite.Require().False(suite.cors.HeadersAllowed([]string{dummyHeader}))

	// add dummyHeader to allowed headers
	suite.cors.AllowHeaders = append(suite.cors.AllowHeaders, dummyHeader)

	// ensure dummyHeader header is allowed
	suite.Require().True(suite.cors.HeadersAllowed([]string{dummyHeader}))
}

func TestCorsSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
